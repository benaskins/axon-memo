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
