package memo

import (
	"context"

	"github.com/benaskins/axon"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = axon.ErrNotFound

// MemoryStore persists memories, extraction jobs, relationship metrics, and personality contexts.
type MemoryStore interface {
	CreateExtractionJob(ctx context.Context, conversationID, agentSlug, userID string) (*ExtractionJob, error)
	UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error
	SaveMemory(ctx context.Context, mem Memory) (string, error)
	SearchMemoriesByVector(ctx context.Context, agentSlug, userID string, embedding []float64, limit int) ([]MemoryWithDistance, error)
	GetOrCreateRelationshipMetrics(ctx context.Context, agentSlug, userID string) (*RelationshipMetrics, error)
	UpdateRelationshipMetrics(ctx context.Context, agentSlug, userID string, shifts map[string]float64) error
	GetUnconsolidatedMemories(ctx context.Context, agentSlug, userID string) ([]Memory, error)
	MarkMemoriesConsolidated(ctx context.Context, memoryIDs []string) error
	SavePersonalityContext(ctx context.Context, agentSlug, userID, context string) error
	GetPersonalityContext(ctx context.Context, agentSlug, userID string) (string, error)
	GetActiveAgents(ctx context.Context) ([]string, error)
	GetActiveUsers(ctx context.Context, agentSlug string) ([]string, error)
}

// ConversationSource reads conversation data. Today this reads directly from
// the chat database; later it can be replaced with an HTTP client.
type ConversationSource interface {
	GetMessages(ctx context.Context, conversationID string) ([]ConversationMessage, error)
	GetAgentInfo(ctx context.Context, agentSlug string) (*AgentInfo, error)
}

// TextGenerator produces text from a prompt. Wired to an LLM at the composition root.
type TextGenerator func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error)

// EmbeddingGenerator produces vector embeddings from text. Wired to an LLM at the composition root.
type EmbeddingGenerator func(ctx context.Context, text string) ([]float64, error)
