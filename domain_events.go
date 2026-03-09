package memo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	fact "github.com/benaskins/axon-fact"
)

// EventTyper is implemented by all domain event structs.
type EventTyper interface {
	EventType() string
}

// NewEvent creates a fact.Event from a domain event struct.
func NewEvent(stream string, data EventTyper) (fact.Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return fact.Event{}, err
	}
	return fact.Event{
		ID:     generateEventID(),
		Stream: stream,
		Type:   data.EventType(),
		Data:   raw,
	}, nil
}

func generateEventID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Stream name helpers

func MemoryStream(agentSlug, userID string) string {
	return "memory-" + agentSlug + "-" + userID
}

// Memory events

type MemoryExtracted struct {
	MemoryID       string  `json:"memory_id"`
	ConversationID string  `json:"conversation_id"`
	AgentSlug      string  `json:"agent_slug"`
	UserID         string  `json:"user_id"`
	MemoryType     string  `json:"memory_type"`
	Content        string  `json:"content"`
	Importance     float64 `json:"importance"`
}

func (e MemoryExtracted) EventType() string { return "memory.extracted" }

type MemoryConsolidated struct {
	SourceMemoryIDs []string `json:"source_memory_ids"`
	NewMemoryID     string   `json:"new_memory_id"`
	AgentSlug       string   `json:"agent_slug"`
	UserID          string   `json:"user_id"`
	Content         string   `json:"content"`
}

func (e MemoryConsolidated) EventType() string { return "memory.consolidated" }

// Relationship events

type RelationshipShifted struct {
	AgentSlug string             `json:"agent_slug"`
	UserID    string             `json:"user_id"`
	Shifts    map[string]float64 `json:"shifts"`
	Reasons   map[string]string  `json:"reasons"`
}

func (e RelationshipShifted) EventType() string { return "relationship.shifted" }

// Personality events

type PersonalitySynthesised struct {
	AgentSlug string `json:"agent_slug"`
	UserID    string `json:"user_id"`
	Context   string `json:"context"`
}

func (e PersonalitySynthesised) EventType() string { return "personality.synthesised" }

// emit appends a domain event to the event store. If es is nil, it's a no-op.
func emit(ctx context.Context, es fact.EventStore, stream string, data EventTyper) error {
	if es == nil {
		return nil
	}
	ev, err := NewEvent(stream, data)
	if err != nil {
		return err
	}
	return es.Append(ctx, stream, []fact.Event{ev})
}
