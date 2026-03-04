package memo

import "time"

// Memory represents a single stored memory with its embedding and metadata.
type Memory struct {
	ID             string
	AgentSlug      string
	UserID         string
	ConversationID *string
	MemoryType     string // episodic, semantic, emotional
	Content        string
	EmotionalTags  *EmotionalTags
	Embedding      []float64
	Importance     float64
	CreatedAt      time.Time
	Consolidated   bool
}

// MemoryWithDistance pairs a Memory with a vector distance score.
type MemoryWithDistance struct {
	Memory
	Distance float64
}

// RelationshipMetrics tracks relationship dynamics between an agent and user.
type RelationshipMetrics struct {
	AgentSlug          string
	UserID             string
	Trust              float64
	Intimacy           float64
	Autonomy           float64
	Reciprocity        float64
	Playfulness        float64
	Conflict           float64
	TotalConversations int
	LastInteraction    *time.Time
	UpdatedAt          time.Time
}

// ExtractionJob tracks the status of a memory extraction operation.
type ExtractionJob struct {
	ID             string
	ConversationID string
	AgentSlug      string
	UserID         string
	Status         string // pending, processing, completed, failed
	Error          *string
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

// EmotionalTags captures the emotional dimensions of a memory.
type EmotionalTags struct {
	Valence  float64  `json:"valence"`  // -1.0 to 1.0
	Arousal  float64  `json:"arousal"`  // 0.0 to 1.0
	Emotions []string `json:"emotions"` // ["joy", "excitement"]
}

// ExtractionResult is the structured output from LLM memory extraction.
type ExtractionResult struct {
	Episodic           []ExtractedMemory      `json:"episodic"`
	Semantic           []ExtractedMemory      `json:"semantic"`
	Emotional          []ExtractedMemory      `json:"emotional"`
	RelationshipShifts map[string]MetricShift `json:"relationship_shifts"`
}

// ExtractedMemory is a single memory extracted by the LLM.
type ExtractedMemory struct {
	Content       string         `json:"content"`
	Importance    float64        `json:"importance"`
	EmotionalTags *EmotionalTags `json:"emotional_tags,omitempty"`
}

// MetricShift describes a change in a relationship metric.
type MetricShift struct {
	Delta  float64 `json:"delta"`
	Reason string  `json:"reason"`
}

// ConsolidationResult is the structured output from LLM memory analysis.
type ConsolidationResult struct {
	Patterns                []Pattern              `json:"patterns"`
	EmotionalArcs           []EmotionalArc         `json:"emotional_arcs"`
	RelationshipEvolution   map[string]MetricShift `json:"relationship_evolution"`
	ConsolidationSuggestions []ConsolidationSuggestion `json:"consolidation_suggestions"`
}

// Pattern identifies a recurring theme across memories.
type Pattern struct {
	Theme        string `json:"theme"`
	Occurrences  int    `json:"occurrences"`
	Significance string `json:"significance"`
	Insight      string `json:"insight"`
}

// EmotionalArc describes an emotional trajectory.
type EmotionalArc struct {
	Arc          string `json:"arc"`
	Significance string `json:"significance"`
}

// ConsolidationSuggestion proposes merging related memories.
type ConsolidationSuggestion struct {
	MemoryIDs           []string `json:"memory_ids"`
	ConsolidatedContent string   `json:"consolidated_content"`
}

// ConversationMessage represents a single message from a conversation.
type ConversationMessage struct {
	Role      string
	Content   string
	CreatedAt time.Time
}

// AgentInfo holds agent identity data needed for memory extraction.
type AgentInfo struct {
	Name         string
	SystemPrompt string
}

// RecallRequest specifies parameters for memory retrieval.
type RecallRequest struct {
	AgentSlug string
	UserID    string
	Query     string
	Limit     int
}

// RecallResponse contains retrieved memories and relationship context.
type RecallResponse struct {
	Memories            []RecalledMemory     `json:"memories"`
	RelationshipContext *RelationshipContext  `json:"relationship_context"`
}

// RecalledMemory is a memory returned from recall with relevance scoring.
type RecalledMemory struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	Content          string    `json:"content"`
	EmotionalContext string    `json:"emotional_context"`
	Timestamp        time.Time `json:"timestamp"`
	Importance       float64   `json:"importance"`
	RelevanceScore   float64   `json:"relevance_score"`
}

// RelationshipContext provides relationship state for recall responses.
type RelationshipContext struct {
	Trust              float64 `json:"trust"`
	Intimacy           float64 `json:"intimacy"`
	Autonomy           float64 `json:"autonomy"`
	Reciprocity        float64 `json:"reciprocity"`
	Playfulness        float64 `json:"playfulness"`
	Conflict           float64 `json:"conflict"`
	PersonalityContext string  `json:"personality_context"`
	TotalConversations int     `json:"total_conversations"`
	TotalMemories      int     `json:"total_memories"`
}

// ExtractRequest is the HTTP request body for memory extraction.
type ExtractRequest struct {
	ConversationID string `json:"conversation_id"`
	AgentSlug      string `json:"agent_slug"`
	UserID         string `json:"user_id"`
}

// ExtractResponse is the HTTP response for memory extraction.
type ExtractResponse struct {
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ConsolidateRequest is the HTTP request body for memory consolidation.
type ConsolidateRequest struct {
	AgentSlug string `json:"agent_slug"`
	UserID    string `json:"user_id"`
}
