package mem

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Extractor orchestrates memory extraction from conversations.
type Extractor struct {
	store    MemoryStore
	source   ConversationSource
	generate TextGenerator
	embed    EmbeddingGenerator
}

// NewExtractor creates an Extractor with the given dependencies.
func NewExtractor(store MemoryStore, source ConversationSource, generate TextGenerator, embed EmbeddingGenerator) *Extractor {
	return &Extractor{
		store:    store,
		source:   source,
		generate: generate,
		embed:    embed,
	}
}

// ExtractConversation runs the full extraction pipeline for a conversation.
func (e *Extractor) ExtractConversation(ctx context.Context, jobID, conversationID, agentSlug, userID string) error {
	if err := e.store.UpdateJobStatus(ctx, jobID, "processing", nil); err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	messages, err := e.source.GetMessages(ctx, conversationID)
	if err != nil {
		errMsg := err.Error()
		e.store.UpdateJobStatus(ctx, jobID, "failed", &errMsg)
		return fmt.Errorf("fetch messages: %w", err)
	}

	if len(messages) == 0 {
		slog.Warn("no messages in conversation", "conversation_id", conversationID)
		errMsg := "no messages found"
		return e.store.UpdateJobStatus(ctx, jobID, "failed", &errMsg)
	}

	agentInfo, err := e.source.GetAgentInfo(ctx, agentSlug)
	if err != nil {
		errMsg := err.Error()
		e.store.UpdateJobStatus(ctx, jobID, "failed", &errMsg)
		return fmt.Errorf("fetch agent: %w", err)
	}

	metrics, err := e.store.GetOrCreateRelationshipMetrics(ctx, agentSlug, userID)
	if err != nil {
		errMsg := err.Error()
		e.store.UpdateJobStatus(ctx, jobID, "failed", &errMsg)
		return fmt.Errorf("get metrics: %w", err)
	}

	prompt := BuildExtractionPrompt(messages, agentInfo, metrics)

	result, err := ExtractMemories(ctx, e.generate, prompt)
	if err != nil {
		errMsg := err.Error()
		e.store.UpdateJobStatus(ctx, jobID, "failed", &errMsg)
		return fmt.Errorf("llm extraction: %w", err)
	}

	if err := e.storeMemories(ctx, result, conversationID, agentSlug, userID); err != nil {
		errMsg := err.Error()
		e.store.UpdateJobStatus(ctx, jobID, "failed", &errMsg)
		return fmt.Errorf("store memories: %w", err)
	}

	if err := e.updateRelationship(ctx, result, agentSlug, userID); err != nil {
		errMsg := err.Error()
		e.store.UpdateJobStatus(ctx, jobID, "failed", &errMsg)
		return fmt.Errorf("update relationship: %w", err)
	}

	if err := e.store.UpdateJobStatus(ctx, jobID, "completed", nil); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	slog.Info("extraction completed",
		"job_id", jobID,
		"conversation_id", conversationID,
		"episodic", len(result.Episodic),
		"semantic", len(result.Semantic),
		"emotional", len(result.Emotional))

	return nil
}

// BuildExtractionPrompt constructs the LLM prompt for memory extraction.
func BuildExtractionPrompt(messages []ConversationMessage, agent *AgentInfo, metrics *RelationshipMetrics) string {
	conversationText := ""
	for _, msg := range messages {
		timestamp := msg.CreatedAt.Format("15:04")
		conversationText += fmt.Sprintf("[%s] %s: %s\n", timestamp, msg.Role, msg.Content)
	}

	prompt := fmt.Sprintf(`You are analyzing a conversation to extract memories and emotional context.

# Conversation Context
Agent: %s
System prompt: %s
Current relationship metrics: trust=%.2f, intimacy=%.2f, autonomy=%.2f, reciprocity=%.2f, playfulness=%.2f, conflict=%.2f

# Conversation History
%s

# Task
Extract memories in three categories:

## EPISODIC MEMORIES (specific events)
What happened in this conversation? Significant moments, shared experiences, events.
Format: [{content: "...", importance: 0.0-1.0, emotional_tags: {valence: -1.0 to 1.0, arousal: 0.0 to 1.0, emotions: [...]}}]

## SEMANTIC MEMORIES (facts learned)
What did you learn about the user? Facts, preferences, beliefs, values, updates.
Format: [{content: "...", importance: 0.0-1.0}]

## EMOTIONAL MEMORIES (feelings)
What emotions were present? User's emotional state, turning points, how they felt.
Format: [{content: "...", importance: 0.0-1.0, emotional_tags: {valence: -1.0 to 1.0, arousal: 0.0 to 1.0, emotions: [...]}}]

## RELATIONSHIP SHIFTS
How did the relationship dynamics change in this conversation?
{
  "trust": {delta: +0.05 or -0.02, reason: "explain why"},
  "intimacy": {delta: ..., reason: ...},
  "autonomy": {delta: ..., reason: ...},
  "reciprocity": {delta: ..., reason: ...},
  "playfulness": {delta: ..., reason: ...},
  "conflict": {delta: ..., reason: ...}
}

Return JSON only. Example:
{
  "episodic": [{"content": "...", "importance": 0.9, "emotional_tags": {"valence": 0.8, "arousal": 0.7, "emotions": ["joy"]}}],
  "semantic": [{"content": "...", "importance": 0.6}],
  "emotional": [{"content": "...", "importance": 0.8, "emotional_tags": {"valence": 0.9, "arousal": 0.5, "emotions": ["validation"]}}],
  "relationship_shifts": {
    "trust": {"delta": 0.03, "reason": "..."},
    "intimacy": {"delta": 0.05, "reason": "..."}
  }
}`,
		agent.Name, agent.SystemPrompt,
		metrics.Trust, metrics.Intimacy, metrics.Autonomy,
		metrics.Reciprocity, metrics.Playfulness, metrics.Conflict,
		conversationText)

	return prompt
}

func (e *Extractor) storeMemories(ctx context.Context, result *ExtractionResult, conversationID, agentSlug, userID string) error {
	now := time.Now()
	var skipped int

	store := func(memType string, mems []ExtractedMemory) error {
		for _, mem := range mems {
			embedding, err := e.embed(ctx, mem.Content)
			if err != nil {
				slog.Warn("failed to generate embedding", "error", err, "content", mem.Content)
				skipped++
				continue
			}

			_, err = e.store.SaveMemory(ctx, Memory{
				AgentSlug:      agentSlug,
				UserID:         userID,
				ConversationID: &conversationID,
				MemoryType:     memType,
				Content:        mem.Content,
				EmotionalTags:  mem.EmotionalTags,
				Embedding:      embedding,
				Importance:     mem.Importance,
				CreatedAt:      now,
				Consolidated:   false,
			})
			if err != nil {
				return fmt.Errorf("save %s memory: %w", memType, err)
			}
		}
		return nil
	}

	if err := store("episodic", result.Episodic); err != nil {
		return err
	}
	if err := store("semantic", result.Semantic); err != nil {
		return err
	}
	if err := store("emotional", result.Emotional); err != nil {
		return err
	}

	if skipped > 0 {
		slog.Warn("skipped memories due to embedding failures", "count", skipped, "conversation_id", conversationID)
	}

	return nil
}

func (e *Extractor) updateRelationship(ctx context.Context, result *ExtractionResult, agentSlug, userID string) error {
	deltas := make(map[string]float64)
	for metric, shift := range result.RelationshipShifts {
		deltas[metric] = shift.Delta
	}
	return e.store.UpdateRelationshipMetrics(ctx, agentSlug, userID, deltas)
}
