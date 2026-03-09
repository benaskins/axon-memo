package memo

import (
	"context"
	"encoding/json"
	"testing"

	fact "github.com/benaskins/axon-fact"
)

func TestExtractor_EmitsMemoryExtractedEvents(t *testing.T) {
	store := &mockStore{savedID: "mem-123"}
	es := fact.NewMemoryStore()

	ext := &Extractor{
		store:      store,
		source:     &stubSource{},
		generate:   stubGenerate,
		embed:      fakeEmbed,
		analytics:  NoopAnalytics{},
		eventStore: es,
	}

	err := ext.ExtractConversation(context.Background(), "job-1", "conv-1", "agent-a", "user-1")
	if err != nil {
		t.Fatalf("extraction failed: %v", err)
	}

	stream := MemoryStream("agent-a", "user-1")
	events, err := es.Load(context.Background(), stream)
	if err != nil {
		t.Fatalf("failed to load events: %v", err)
	}

	// stubGenerate returns 1 episodic + 1 semantic memory, plus a relationship shift
	var extracted, shifted int
	for _, ev := range events {
		switch ev.Type {
		case "memory.extracted":
			extracted++
			var data MemoryExtracted
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				t.Fatalf("failed to unmarshal memory.extracted: %v", err)
			}
			if data.MemoryID != "mem-123" {
				t.Errorf("expected memory ID mem-123, got %s", data.MemoryID)
			}
			if data.ConversationID != "conv-1" {
				t.Errorf("expected conversation ID conv-1, got %s", data.ConversationID)
			}
		case "relationship.shifted":
			shifted++
			var data RelationshipShifted
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				t.Fatalf("failed to unmarshal relationship.shifted: %v", err)
			}
			if data.Shifts["ability"] != 0.05 {
				t.Errorf("expected ability shift 0.05, got %f", data.Shifts["ability"])
			}
			if data.Reasons["ability"] != "showed competence" {
				t.Errorf("expected reason 'showed competence', got %s", data.Reasons["ability"])
			}
		}
	}

	if extracted != 2 {
		t.Errorf("expected 2 memory.extracted events, got %d", extracted)
	}
	if shifted != 1 {
		t.Errorf("expected 1 relationship.shifted event, got %d", shifted)
	}
}

func TestExtractor_NoEventsWithoutEventStore(t *testing.T) {
	store := &mockStore{savedID: "mem-123"}

	ext := &Extractor{
		store:     store,
		source:    &stubSource{},
		generate:  stubGenerate,
		embed:     fakeEmbed,
		analytics: NoopAnalytics{},
		// eventStore deliberately nil
	}

	err := ext.ExtractConversation(context.Background(), "job-1", "conv-1", "agent-a", "user-1")
	if err != nil {
		t.Fatalf("extraction failed: %v", err)
	}

	// No panic, no error — events are silently skipped
}

func TestConsolidator_EmitsConsolidationEvents(t *testing.T) {
	store := &mockStore{
		savedID: "consolidated-1",
		metrics: &RelationshipMetrics{Ability: 0.5, Benevolence: 0.5, Integrity: 0.5},
	}
	es := fact.NewMemoryStore()

	c := &Consolidator{
		store:      store,
		source:     &stubSource{},
		generate:   stubConsolidateGenerate,
		embed:      fakeEmbed,
		analytics:  NoopAnalytics{},
		eventStore: es,
	}

	// Provide unconsolidated memories
	store.unconsolidated = []Memory{
		{ID: "mem-1", MemoryType: "episodic", Content: "test memory", Importance: 0.7},
	}

	err := c.ConsolidateAgent(context.Background(), "agent-a", "user-1")
	if err != nil {
		t.Fatalf("consolidation failed: %v", err)
	}

	stream := MemoryStream("agent-a", "user-1")
	events, err := es.Load(context.Background(), stream)
	if err != nil {
		t.Fatalf("failed to load events: %v", err)
	}

	types := map[string]int{}
	for _, ev := range events {
		types[ev.Type]++
	}

	if types["memory.consolidated"] != 1 {
		t.Errorf("expected 1 memory.consolidated event, got %d", types["memory.consolidated"])
	}
	if types["relationship.shifted"] != 1 {
		t.Errorf("expected 1 relationship.shifted event, got %d", types["relationship.shifted"])
	}
	if types["personality.synthesised"] != 1 {
		t.Errorf("expected 1 personality.synthesised event, got %d", types["personality.synthesised"])
	}
}

// --- Test doubles ---

type stubSource struct{}

func (s *stubSource) GetMessages(_ context.Context, _ string) ([]ConversationMessage, error) {
	return []ConversationMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}, nil
}

func (s *stubSource) GetAgentInfo(_ context.Context, _ string) (*AgentInfo, error) {
	return &AgentInfo{Name: "Test Agent", SystemPrompt: "Be helpful"}, nil
}

// stubGenerate returns a fixed extraction result with 1 episodic + 1 semantic memory
// and a relationship shift.
func stubGenerate(_ context.Context, prompt string, _ float64, _ int) (string, error) {
	return `{
		"episodic": [{"content": "user said hello", "importance": 0.6}],
		"semantic": [{"content": "user is friendly", "importance": 0.7}],
		"emotional": [],
		"relationship_shifts": {
			"ability": {"delta": 0.05, "reason": "showed competence"}
		}
	}`, nil
}

// stubConsolidateGenerate returns a fixed consolidation result.
func stubConsolidateGenerate(_ context.Context, prompt string, _ float64, _ int) (string, error) {
	return `{
		"patterns": [{"theme": "greetings", "occurrences": 2, "significance": "low", "insight": "user greets often"}],
		"emotional_arcs": [],
		"relationship_evolution": {
			"benevolence": {"delta": 0.03, "reason": "consistent care"}
		},
		"consolidation_suggestions": [
			{"memory_ids": ["mem-1"], "consolidated_content": "user frequently greets warmly"}
		]
	}`, nil
}
