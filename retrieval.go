package memo

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
)

// Retriever handles memory recall with vector search and reranking.
type Retriever struct {
	store MemoryStore
	embed EmbeddingGenerator
}

// NewRetriever creates a Retriever with the given dependencies.
func NewRetriever(store MemoryStore, embed EmbeddingGenerator) *Retriever {
	return &Retriever{
		store: store,
		embed: embed,
	}
}

// Recall retrieves and ranks memories relevant to a query.
func (r *Retriever) Recall(ctx context.Context, req RecallRequest) (*RecallResponse, error) {
	if req.Limit == 0 {
		req.Limit = 5
	}

	embedding, err := r.embed(ctx, req.Query)
	if err != nil {
		return nil, err
	}

	candidateLimit := req.Limit * 4
	if candidateLimit < 20 {
		candidateLimit = 20
	}

	candidates, err := r.store.SearchMemoriesByVector(ctx, req.AgentSlug, req.UserID, embedding, candidateLimit)
	if err != nil {
		return nil, err
	}

	scored := Rerank(candidates)
	selected := BalanceTypes(scored, req.Limit)

	relCtx, err := r.getRelationshipContext(ctx, req.AgentSlug, req.UserID, len(candidates))
	if err != nil {
		return nil, err
	}

	return &RecallResponse{
		Memories:            selected,
		RelationshipContext: relCtx,
	}, nil
}

// Rerank scores and sorts memories by combining semantic relevance,
// importance, recency, and emotional arousal.
func Rerank(candidates []MemoryWithDistance) []RecalledMemory {
	now := time.Now()
	scored := make([]RecalledMemory, 0, len(candidates))

	for _, candidate := range candidates {
		semanticRelevance := 1.0 - candidate.Distance
		importanceWeight := candidate.Importance

		var finalScore float64
		if candidate.Durable {
			finalScore = semanticRelevance * importanceWeight
		} else {
			daysSince := now.Sub(candidate.CreatedAt).Hours() / 24
			recencyBoost := math.Exp(-daysSince / 30.0)

			emotionalBoost := 1.0
			if candidate.EmotionalTags != nil {
				emotionalBoost = 1.0 + (candidate.EmotionalTags.Arousal * 0.5)
			}

			finalScore = semanticRelevance * importanceWeight * recencyBoost * emotionalBoost
		}

		emotionalContext := ""
		if candidate.EmotionalTags != nil && len(candidate.EmotionalTags.Emotions) > 0 {
			emotionalContext = strings.Join(candidate.EmotionalTags.Emotions, ", ")
		}

		scored = append(scored, RecalledMemory{
			ID:               candidate.ID,
			Type:             candidate.MemoryType,
			Content:          candidate.Content,
			EmotionalContext: emotionalContext,
			Timestamp:        candidate.CreatedAt,
			Importance:       candidate.Importance,
			RelevanceScore:   finalScore,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].RelevanceScore > scored[j].RelevanceScore
	})

	return scored
}

// BalanceTypes ensures diversity across memory types in the selected results.
func BalanceTypes(scored []RecalledMemory, limit int) []RecalledMemory {
	if len(scored) <= limit {
		return scored
	}

	emotional := []RecalledMemory{}
	episodic := []RecalledMemory{}
	semantic := []RecalledMemory{}

	for _, mem := range scored {
		switch mem.Type {
		case "emotional":
			emotional = append(emotional, mem)
		case "episodic":
			episodic = append(episodic, mem)
		case "semantic":
			semantic = append(semantic, mem)
		}
	}

	// Ensure at least 1 of each type (if available)
	selected := []RecalledMemory{}

	if len(emotional) > 0 {
		selected = append(selected, emotional[0])
	}
	if len(episodic) > 0 && len(selected) < limit {
		selected = append(selected, episodic[0])
	}
	if len(semantic) > 0 && len(selected) < limit {
		selected = append(selected, semantic[0])
	}

	if len(selected) >= limit {
		return selected[:limit]
	}

	// Fill remaining slots with highest-scoring
	for _, mem := range scored {
		if len(selected) >= limit {
			break
		}

		alreadySelected := false
		for _, s := range selected {
			if s.ID == mem.ID {
				alreadySelected = true
				break
			}
		}

		if !alreadySelected {
			selected = append(selected, mem)
		}
	}

	return selected
}

func (r *Retriever) getRelationshipContext(ctx context.Context, agentSlug, userID string, totalMemories int) (*RelationshipContext, error) {
	metrics, err := r.store.GetOrCreateRelationshipMetrics(ctx, agentSlug, userID)
	if err != nil {
		return nil, err
	}

	personalityContext, err := r.store.GetPersonalityContext(ctx, agentSlug, userID)
	if err != nil {
		slog.Warn("failed to get personality context", "error", err)
		personalityContext = ""
	}

	return &RelationshipContext{
		Ability:            metrics.Ability,
		Benevolence:        metrics.Benevolence,
		Integrity:          metrics.Integrity,
		PersonalityContext: personalityContext,
		TotalConversations: metrics.TotalConversations,
		TotalMemories:      totalMemories,
	}, nil
}
