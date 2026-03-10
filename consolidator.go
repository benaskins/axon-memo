package memo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	fact "github.com/benaskins/axon-fact"
)

// Consolidator analyzes and merges memories overnight.
type Consolidator struct {
	store      MemoryStore
	source     ConversationSource
	generate   TextGenerator
	embed      EmbeddingGenerator
	analytics  AnalyticsEmitter
	eventStore fact.EventStore
}

// NewConsolidator creates a Consolidator with the given dependencies.
func NewConsolidator(store MemoryStore, source ConversationSource, generate TextGenerator, embed EmbeddingGenerator) *Consolidator {
	return &Consolidator{
		store:     store,
		source:    source,
		generate:  generate,
		embed:     embed,
		analytics: NoopAnalytics{},
	}
}

// ConsolidateAll runs consolidation for all agents with recent activity.
func (c *Consolidator) ConsolidateAll(ctx context.Context) error {
	agents, err := c.store.GetActiveAgents(ctx)
	if err != nil {
		return fmt.Errorf("get active agents: %w", err)
	}

	slog.Info("starting overnight consolidation", "agents", len(agents))

	for _, agentSlug := range agents {
		users, err := c.store.GetActiveUsers(ctx, agentSlug)
		if err != nil {
			slog.Error("failed to get active users", "agent", agentSlug, "error", err)
			continue
		}

		for _, userID := range users {
			if err := c.ConsolidateAgent(ctx, agentSlug, userID); err != nil {
				slog.Error("consolidation failed", "agent", agentSlug, "user", userID, "error", err)
				continue
			}
		}
	}

	slog.Info("overnight consolidation complete", "agents_processed", len(agents))
	return nil
}

// ConsolidateAgent runs consolidation for a single agent-user pair.
func (c *Consolidator) ConsolidateAgent(ctx context.Context, agentSlug, userID string) error {
	memories, err := c.store.GetUnconsolidatedMemories(ctx, agentSlug, userID)
	if err != nil {
		return fmt.Errorf("get unconsolidated memories: %w", err)
	}

	if len(memories) == 0 {
		slog.Info("no memories to consolidate", "agent", agentSlug, "user", userID)
		return nil
	}

	metrics, err := c.store.GetOrCreateRelationshipMetrics(ctx, agentSlug, userID)
	if err != nil {
		return fmt.Errorf("get relationship metrics: %w", err)
	}

	prompt := BuildAnalysisPrompt(memories, agentSlug, metrics)

	result, err := AnalyzeMemories(ctx, c.generate, prompt)
	if err != nil {
		return fmt.Errorf("llm analysis: %w", err)
	}

	if err := c.applyConsolidation(ctx, result, agentSlug, userID); err != nil {
		return fmt.Errorf("apply consolidation: %w", err)
	}

	if err := c.applyRelationshipEvolution(ctx, result, agentSlug, userID); err != nil {
		return fmt.Errorf("apply relationship evolution: %w", err)
	}

	if err := c.generatePersonality(ctx, agentSlug, userID, metrics, result); err != nil {
		return fmt.Errorf("generate personality: %w", err)
	}

	// Emit analytics
	c.analytics.Emit(ConsolidationCompletedEvent(agentSlug, userID, len(result.Patterns), len(result.ConsolidationSuggestions)))

	slog.Info("consolidation complete",
		"agent", agentSlug,
		"user", userID,
		"memories_processed", len(memories),
		"patterns", len(result.Patterns),
		"consolidations", len(result.ConsolidationSuggestions))

	return nil
}

// BuildAnalysisPrompt constructs the LLM prompt for memory consolidation.
func BuildAnalysisPrompt(memories []Memory, agentSlug string, metrics *RelationshipMetrics) string {
	memoryText := ""
	for _, mem := range memories {
		timestamp := mem.CreatedAt.Format("2006-01-02 15:04")
		memoryText += fmt.Sprintf("[%s] %s (%s, importance: %.2f): %s\n",
			timestamp, mem.MemoryType, mem.ID, mem.Importance, mem.Content)
	}

	prompt := fmt.Sprintf(`You are reviewing a day's worth of memories for agent %s.

# Today's Memories (unconsolidated)
%s

# Current Trustworthiness Metrics (Mayer et al. 1995)
Ability: %.2f (competence in the relevant domain)
Benevolence: %.2f (positive orientation toward the user's interests)
Integrity: %.2f (adherence to acceptable principles)

# Task
Identify patterns and themes:

## RECURRING THEMES
What topics, concerns, or subjects came up multiple times?

## EMOTIONAL PATTERNS
What emotional arcs or patterns emerged? (e.g., "started anxious, ended relieved")

## TRUSTWORTHINESS SHIFTS
How did the trustworthiness dimensions evolve today?

## BEHAVIORAL CHANGES
Any changes in how the user is interacting?

## CONSOLIDATION OPPORTUNITIES
Which similar memories can be merged into higher-level semantic memories?

Return JSON:
{
  "patterns": [{"theme": "...", "occurrences": 3, "significance": "high|medium|low", "insight": "..."}],
  "emotional_arcs": [{"arc": "...", "significance": "high|medium|low"}],
  "relationship_evolution": {"ability": {"delta": 0.05, "reason": "..."}, ...},
  "consolidation_suggestions": [
    {
      "memory_ids": ["uuid1", "uuid2"],
      "consolidated_content": "A single string describing the merged memory pattern (not a JSON object)"
    }
  ]
}`,
		agentSlug, memoryText,
		metrics.Ability, metrics.Benevolence, metrics.Integrity)

	return prompt
}

func (c *Consolidator) applyConsolidation(ctx context.Context, result *ConsolidationResult, agentSlug, userID string) error {
	now := time.Now()
	stream := MemoryStream(agentSlug, userID)

	for _, suggestion := range result.ConsolidationSuggestions {
		embedding, err := c.embed(ctx, suggestion.ConsolidatedContent)
		if err != nil {
			slog.Warn("failed to generate embedding for consolidated memory, skipping", "error", err)
			continue
		}

		id, err := c.store.SaveMemory(ctx, Memory{
			AgentSlug:      agentSlug,
			UserID:         userID,
			ConversationID: nil,
			MemoryType:     "semantic",
			Content:        suggestion.ConsolidatedContent,
			Embedding:      embedding,
			Importance:     0.85,
			CreatedAt:      now,
			Consolidated:   true,
		})

		if err != nil {
			return fmt.Errorf("save consolidated memory: %w", err)
		}

		// Mark source memories as consolidated AFTER the replacement is saved
		// to prevent data loss if the save fails.
		if err := c.store.MarkMemoriesConsolidated(ctx, suggestion.MemoryIDs); err != nil {
			return fmt.Errorf("mark consolidated: %w", err)
		}

		if err := emit(ctx, c.eventStore, stream, MemoryConsolidated{
			SourceMemoryIDs: suggestion.MemoryIDs,
			NewMemoryID:     id,
			AgentSlug:       agentSlug,
			UserID:          userID,
			Content:         suggestion.ConsolidatedContent,
		}); err != nil {
			slog.Warn("failed to emit memory.consolidated event", "error", err)
		}
	}

	return nil
}

func (c *Consolidator) applyRelationshipEvolution(ctx context.Context, result *ConsolidationResult, agentSlug, userID string) error {
	deltas := make(map[string]float64)
	reasons := make(map[string]string)
	for metric, shift := range result.RelationshipEvolution {
		deltas[metric] = shift.Delta
		reasons[metric] = shift.Reason
	}
	if len(deltas) == 0 {
		return nil
	}
	if err := c.store.UpdateRelationshipMetrics(ctx, agentSlug, userID, deltas); err != nil {
		return err
	}
	if err := emit(ctx, c.eventStore, MemoryStream(agentSlug, userID), RelationshipShifted{
		AgentSlug: agentSlug,
		UserID:    userID,
		Shifts:    deltas,
		Reasons:   reasons,
	}); err != nil {
		slog.Warn("failed to emit relationship.shifted event", "error", err)
	}
	return nil
}

func (c *Consolidator) generatePersonality(ctx context.Context, agentSlug, userID string, metrics *RelationshipMetrics, result *ConsolidationResult) error {
	agentInfo, err := c.source.GetAgentInfo(ctx, agentSlug)
	if err != nil {
		return fmt.Errorf("fetch agent info: %w", err)
	}

	prompt := BuildPersonalityPrompt(agentInfo.Name, agentInfo.SystemPrompt, metrics, result)

	personalityContext, err := GeneratePersonalityContext(ctx, c.generate, prompt)
	if err != nil {
		return fmt.Errorf("generate personality: %w", err)
	}

	if err := c.store.SavePersonalityContext(ctx, agentSlug, userID, personalityContext); err != nil {
		return err
	}

	if err := emit(ctx, c.eventStore, MemoryStream(agentSlug, userID), PersonalitySynthesised{
		AgentSlug: agentSlug,
		UserID:    userID,
		Context:   personalityContext,
	}); err != nil {
		slog.Warn("failed to emit personality.synthesised event", "error", err)
	}
	return nil
}

// BuildPersonalityPrompt constructs the LLM prompt for personality generation.
func BuildPersonalityPrompt(agentName, systemPrompt string, metrics *RelationshipMetrics, result *ConsolidationResult) string {
	patternText := ""
	for _, p := range result.Patterns {
		patternText += fmt.Sprintf("- %s (%s significance)\n", p.Insight, p.Significance)
	}

	prompt := fmt.Sprintf(`Based on accumulated memories and trustworthiness metrics, generate a personality context for the agent.

# Agent Definition
Name: %s
System prompt: %s

# Trustworthiness Metrics (Mayer et al. 1995)
Ability: %.2f (%.0f%%) — competence in the relevant domain
Benevolence: %.2f (%.0f%%) — positive orientation toward the user's interests
Integrity: %.2f (%.0f%%) — adherence to acceptable principles

# Recent Patterns
%s

# Task
Generate a personality modifier (200-300 words) that:
1. Describes the evolved relationship naturally
2. Guides agent's tone, approach, and behavior
3. Reflects growth and changes over time
4. Adjusts based on current trust dynamics

Write in second person ("You've developed..."), as instructions to the agent.
Return plain text (not JSON).`,
		agentName, systemPrompt,
		metrics.Ability, metrics.Ability*100,
		metrics.Benevolence, metrics.Benevolence*100,
		metrics.Integrity, metrics.Integrity*100,
		patternText)

	return prompt
}
