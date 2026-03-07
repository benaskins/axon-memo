package memo

import (
	"testing"
	"time"
)

func TestRerank(t *testing.T) {
	now := time.Now()
	candidates := []MemoryWithDistance{
		{
			Memory: Memory{
				ID:         "1",
				MemoryType: "semantic",
				Content:    "Old memory",
				Importance: 0.5,
				CreatedAt:  now.Add(-60 * 24 * time.Hour), // 60 days old
			},
			Distance: 0.3, // Good semantic match
		},
		{
			Memory: Memory{
				ID:         "2",
				MemoryType: "emotional",
				Content:    "Recent emotional memory",
				Importance: 0.8,
				EmotionalTags: &EmotionalTags{
					Arousal:  0.9,
					Emotions: []string{"joy", "excitement"},
				},
				CreatedAt: now.Add(-2 * 24 * time.Hour), // 2 days old
			},
			Distance: 0.4, // Moderate semantic match
		},
	}

	scored := Rerank(candidates)

	if len(scored) != 2 {
		t.Fatalf("expected 2 scored memories, got %d", len(scored))
	}

	// Recent emotional memory should score higher despite worse semantic match
	if scored[0].ID != "2" {
		t.Errorf("expected emotional memory to rank first, got %s", scored[0].ID)
	}

	if scored[0].RelevanceScore <= scored[1].RelevanceScore {
		t.Errorf("expected first memory to have higher score: %f vs %f",
			scored[0].RelevanceScore, scored[1].RelevanceScore)
	}
}

func TestRerank_DurableMemorySkipsDecay(t *testing.T) {
	now := time.Now()
	candidates := []MemoryWithDistance{
		{
			Memory: Memory{
				ID:         "recent",
				MemoryType: "episodic",
				Content:    "Recent ephemeral memory",
				Importance: 0.7,
				CreatedAt:  now.Add(-2 * 24 * time.Hour), // 2 days old
			},
			Distance: 0.3,
		},
		{
			Memory: Memory{
				ID:         "old-durable",
				MemoryType: "semantic",
				Content:    "Pull-based coordination, not assignment",
				Importance: 0.9,
				Durable:    true,
				CreatedAt:  now.Add(-180 * 24 * time.Hour), // 6 months old
			},
			Distance: 0.3, // Same semantic match
		},
	}

	scored := Rerank(candidates)

	// Durable memory should rank higher despite being 6 months old,
	// because it has higher importance and no recency decay
	if scored[0].ID != "old-durable" {
		t.Errorf("expected durable memory to rank first, got %s (scores: %f vs %f)",
			scored[0].ID, scored[0].RelevanceScore, scored[1].RelevanceScore)
	}
}

func TestBalanceTypes(t *testing.T) {
	scored := []RecalledMemory{
		{ID: "1", Type: "semantic", RelevanceScore: 0.9},
		{ID: "2", Type: "semantic", RelevanceScore: 0.8},
		{ID: "3", Type: "episodic", RelevanceScore: 0.7},
		{ID: "4", Type: "emotional", RelevanceScore: 0.6},
		{ID: "5", Type: "semantic", RelevanceScore: 0.5},
	}

	selected := BalanceTypes(scored, 3)

	if len(selected) != 3 {
		t.Fatalf("expected 3 selected memories, got %d", len(selected))
	}

	types := make(map[string]int)
	for _, mem := range selected {
		types[mem.Type]++
	}

	if types["emotional"] == 0 {
		t.Error("expected at least one emotional memory")
	}
	if types["episodic"] == 0 {
		t.Error("expected at least one episodic memory")
	}
	if types["semantic"] == 0 {
		t.Error("expected at least one semantic memory")
	}
}

func TestBalanceTypes_EmptyInput(t *testing.T) {
	selected := BalanceTypes(nil, 5)
	if len(selected) != 0 {
		t.Errorf("expected 0 memories from nil input, got %d", len(selected))
	}

	selected = BalanceTypes([]RecalledMemory{}, 5)
	if len(selected) != 0 {
		t.Errorf("expected 0 memories from empty input, got %d", len(selected))
	}
}

func TestBalanceTypes_AllSameType(t *testing.T) {
	scored := []RecalledMemory{
		{ID: "1", Type: "semantic", RelevanceScore: 0.9},
		{ID: "2", Type: "semantic", RelevanceScore: 0.8},
		{ID: "3", Type: "semantic", RelevanceScore: 0.7},
		{ID: "4", Type: "semantic", RelevanceScore: 0.6},
		{ID: "5", Type: "semantic", RelevanceScore: 0.5},
	}

	selected := BalanceTypes(scored, 3)

	if len(selected) != 3 {
		t.Fatalf("expected 3 selected memories, got %d", len(selected))
	}

	// With only one type available, first slot goes to semantic[0],
	// then fill remaining with highest-scoring unselected
	if selected[0].ID != "1" {
		t.Errorf("expected first selected to be ID 1, got %s", selected[0].ID)
	}
	if selected[1].ID != "2" {
		t.Errorf("expected second selected to be ID 2, got %s", selected[1].ID)
	}
	if selected[2].ID != "3" {
		t.Errorf("expected third selected to be ID 3, got %s", selected[2].ID)
	}
}

func TestBalanceTypes_FewerCandidatesThanLimit(t *testing.T) {
	scored := []RecalledMemory{
		{ID: "1", Type: "semantic", RelevanceScore: 0.9},
		{ID: "2", Type: "episodic", RelevanceScore: 0.7},
	}

	selected := BalanceTypes(scored, 10)

	// When candidates <= limit, return all candidates unchanged
	if len(selected) != 2 {
		t.Fatalf("expected 2 memories (all candidates), got %d", len(selected))
	}
	if selected[0].ID != "1" || selected[1].ID != "2" {
		t.Error("expected candidates returned in original order")
	}
}

func TestBalanceTypes_ExactlyAtLimit(t *testing.T) {
	scored := []RecalledMemory{
		{ID: "1", Type: "semantic", RelevanceScore: 0.9},
		{ID: "2", Type: "episodic", RelevanceScore: 0.7},
		{ID: "3", Type: "emotional", RelevanceScore: 0.5},
	}

	selected := BalanceTypes(scored, 3)

	// When candidates == limit, return all candidates unchanged
	if len(selected) != 3 {
		t.Fatalf("expected 3 memories, got %d", len(selected))
	}
}

func TestBalanceTypes_LimitOne(t *testing.T) {
	scored := []RecalledMemory{
		{ID: "1", Type: "semantic", RelevanceScore: 0.9},
		{ID: "2", Type: "episodic", RelevanceScore: 0.8},
		{ID: "3", Type: "emotional", RelevanceScore: 0.7},
	}

	selected := BalanceTypes(scored, 1)

	if len(selected) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(selected))
	}
}

func TestBalanceTypes_TwoTypesOnly(t *testing.T) {
	scored := []RecalledMemory{
		{ID: "1", Type: "semantic", RelevanceScore: 0.9},
		{ID: "2", Type: "episodic", RelevanceScore: 0.8},
		{ID: "3", Type: "semantic", RelevanceScore: 0.7},
		{ID: "4", Type: "episodic", RelevanceScore: 0.6},
	}

	selected := BalanceTypes(scored, 3)

	if len(selected) != 3 {
		t.Fatalf("expected 3 memories, got %d", len(selected))
	}

	types := make(map[string]int)
	for _, mem := range selected {
		types[mem.Type]++
	}

	if types["semantic"] == 0 {
		t.Error("expected at least one semantic memory")
	}
	if types["episodic"] == 0 {
		t.Error("expected at least one episodic memory")
	}
}
