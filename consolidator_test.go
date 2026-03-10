package memo

import (
	"strings"
	"testing"
	"time"
)

func TestBuildAnalysisPrompt(t *testing.T) {
	now := time.Date(2026, 3, 7, 14, 30, 0, 0, time.UTC)
	memories := []Memory{
		{
			ID:         "mem-1",
			MemoryType: "episodic",
			Content:    "User asked about Go concurrency",
			Importance: 0.7,
			CreatedAt:  now,
		},
		{
			ID:         "mem-2",
			MemoryType: "emotional",
			Content:    "User expressed frustration with debugging",
			Importance: 0.9,
			CreatedAt:  now.Add(30 * time.Minute),
		},
	}

	metrics := &RelationshipMetrics{
		Ability:     0.75,
		Benevolence: 0.60,
		Integrity:   0.85,
	}

	prompt := BuildAnalysisPrompt(memories, "test-agent", metrics)

	// Verify agent slug appears
	if !strings.Contains(prompt, "test-agent") {
		t.Error("prompt should contain agent slug")
	}

	// Verify memories are formatted with timestamp, type, ID, importance, content
	if !strings.Contains(prompt, "[2026-03-07 14:30] episodic (mem-1, importance: 0.70): User asked about Go concurrency") {
		t.Error("prompt should contain formatted first memory")
	}
	if !strings.Contains(prompt, "[2026-03-07 15:00] emotional (mem-2, importance: 0.90): User expressed frustration with debugging") {
		t.Error("prompt should contain formatted second memory")
	}

	// Verify metrics are included
	if !strings.Contains(prompt, "Ability: 0.75") {
		t.Error("prompt should contain ability metric")
	}
	if !strings.Contains(prompt, "Benevolence: 0.60") {
		t.Error("prompt should contain benevolence metric")
	}
	if !strings.Contains(prompt, "Integrity: 0.85") {
		t.Error("prompt should contain integrity metric")
	}

	// Verify structural sections exist
	for _, section := range []string{
		"RECURRING THEMES",
		"EMOTIONAL PATTERNS",
		"TRUSTWORTHINESS SHIFTS",
		"BEHAVIORAL CHANGES",
		"CONSOLIDATION OPPORTUNITIES",
	} {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt should contain section %q", section)
		}
	}

	// Verify JSON schema hint is present
	if !strings.Contains(prompt, `"patterns"`) {
		t.Error("prompt should contain JSON schema with patterns key")
	}
}

func TestBuildAnalysisPrompt_EmptyMemories(t *testing.T) {
	metrics := &RelationshipMetrics{
		Ability:     0.50,
		Benevolence: 0.50,
		Integrity:   0.50,
	}

	prompt := BuildAnalysisPrompt(nil, "empty-agent", metrics)

	if !strings.Contains(prompt, "empty-agent") {
		t.Error("prompt should still contain agent slug with no memories")
	}
	if !strings.Contains(prompt, "Ability: 0.50") {
		t.Error("prompt should still contain metrics with no memories")
	}
}

func TestBuildPersonalityPrompt(t *testing.T) {
	metrics := &RelationshipMetrics{
		Ability:     0.80,
		Benevolence: 0.65,
		Integrity:   0.90,
	}

	result := &ConsolidationResult{
		Patterns: []Pattern{
			{Insight: "User prefers concise answers", Significance: "high"},
			{Insight: "Frequent Go questions", Significance: "medium"},
		},
	}

	prompt := BuildPersonalityPrompt("TestBot", "You are a helpful assistant.", metrics, result)

	// Verify agent identity
	if !strings.Contains(prompt, "Name: TestBot") {
		t.Error("prompt should contain agent name")
	}
	if !strings.Contains(prompt, "System prompt: You are a helpful assistant.") {
		t.Error("prompt should contain system prompt")
	}

	// Verify metrics with percentages
	if !strings.Contains(prompt, "Ability: 0.80 (80%)") {
		t.Error("prompt should contain ability with percentage")
	}
	if !strings.Contains(prompt, "Benevolence: 0.65 (65%)") {
		t.Error("prompt should contain benevolence with percentage")
	}
	if !strings.Contains(prompt, "Integrity: 0.90 (90%)") {
		t.Error("prompt should contain integrity with percentage")
	}

	// Verify patterns are included
	if !strings.Contains(prompt, "User prefers concise answers (high significance)") {
		t.Error("prompt should contain first pattern")
	}
	if !strings.Contains(prompt, "Frequent Go questions (medium significance)") {
		t.Error("prompt should contain second pattern")
	}

	// Verify task instructions
	if !strings.Contains(prompt, "personality modifier") {
		t.Error("prompt should describe the task")
	}
	if !strings.Contains(prompt, "second person") {
		t.Error("prompt should instruct second-person writing")
	}
}

func TestBuildPersonalityPrompt_NoPatterns(t *testing.T) {
	metrics := &RelationshipMetrics{
		Ability:     0.50,
		Benevolence: 0.50,
		Integrity:   0.50,
	}

	result := &ConsolidationResult{
		Patterns: nil,
	}

	prompt := BuildPersonalityPrompt("EmptyBot", "Be helpful.", metrics, result)

	if !strings.Contains(prompt, "Name: EmptyBot") {
		t.Error("prompt should contain agent name even with no patterns")
	}
	// Should still have the Recent Patterns section header
	if !strings.Contains(prompt, "Recent Patterns") {
		t.Error("prompt should contain Recent Patterns section even when empty")
	}
}
