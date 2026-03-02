package mem

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractJSONFromMarkdown_JSONBlock(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}\n```"
	got := extractJSONFromMarkdown(input)
	if got != `{"key": "value"}` {
		t.Errorf("expected JSON content, got %q", got)
	}
}

func TestExtractJSONFromMarkdown_PlainBlock(t *testing.T) {
	input := "```\n{\"key\": \"value\"}\n```"
	got := extractJSONFromMarkdown(input)
	if got != `{"key": "value"}` {
		t.Errorf("expected JSON content, got %q", got)
	}
}

func TestExtractJSONFromMarkdown_MissingClosing(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}"
	got := extractJSONFromMarkdown(input)
	if got != `{"key": "value"}` {
		t.Errorf("expected JSON content, got %q", got)
	}
}

func TestExtractJSONFromMarkdown_NoFences(t *testing.T) {
	input := `{"key": "value"}`
	got := extractJSONFromMarkdown(input)
	if got != input {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestExtractMemories_ValidJSON(t *testing.T) {
	result := ExtractionResult{
		Episodic: []ExtractedMemory{
			{Content: "User mentioned they like hiking", Importance: 0.7},
		},
		Semantic: []ExtractedMemory{
			{Content: "User prefers outdoor activities", Importance: 0.6},
		},
		Emotional: []ExtractedMemory{
			{
				Content:    "User felt excited about the weekend trip",
				Importance: 0.8,
				EmotionalTags: &EmotionalTags{
					Valence:  0.9,
					Arousal:  0.7,
					Emotions: []string{"joy", "excitement"},
				},
			},
		},
		RelationshipShifts: map[string]MetricShift{
			"trust": {Delta: 0.1, Reason: "User shared personal plans"},
		},
	}

	resultJSON, _ := json.Marshal(result)
	generate := func(_ context.Context, _ string, _ float64, _ int) (string, error) {
		return string(resultJSON), nil
	}

	got, err := ExtractMemories(context.Background(), generate, "test prompt")
	if err != nil {
		t.Fatalf("ExtractMemories failed: %v", err)
	}

	if len(got.Episodic) != 1 {
		t.Errorf("expected 1 episodic memory, got %d", len(got.Episodic))
	}
	if len(got.Semantic) != 1 {
		t.Errorf("expected 1 semantic memory, got %d", len(got.Semantic))
	}
	if len(got.Emotional) != 1 {
		t.Errorf("expected 1 emotional memory, got %d", len(got.Emotional))
	}
	if got.Emotional[0].EmotionalTags == nil {
		t.Fatal("expected emotional tags, got nil")
	}
	if got.Emotional[0].EmotionalTags.Valence != 0.9 {
		t.Errorf("expected valence 0.9, got %f", got.Emotional[0].EmotionalTags.Valence)
	}
	if shift, ok := got.RelationshipShifts["trust"]; !ok {
		t.Error("expected trust shift")
	} else if shift.Delta != 0.1 {
		t.Errorf("expected delta 0.1, got %f", shift.Delta)
	}
}

func TestExtractMemories_MarkdownCodeBlock(t *testing.T) {
	result := ExtractionResult{
		Episodic: []ExtractedMemory{
			{Content: "Test memory", Importance: 0.5},
		},
	}
	resultJSON, _ := json.Marshal(result)
	content := "```json\n" + string(resultJSON) + "\n```"

	generate := func(_ context.Context, _ string, _ float64, _ int) (string, error) {
		return content, nil
	}

	got, err := ExtractMemories(context.Background(), generate, "test prompt")
	if err != nil {
		t.Fatalf("ExtractMemories failed: %v", err)
	}
	if len(got.Episodic) != 1 {
		t.Errorf("expected 1 episodic memory, got %d", len(got.Episodic))
	}
	if got.Episodic[0].Content != "Test memory" {
		t.Errorf("expected content 'Test memory', got '%s'", got.Episodic[0].Content)
	}
}

func TestExtractMemories_MarkdownCodeBlockMissingClosing(t *testing.T) {
	result := ExtractionResult{
		Episodic: []ExtractedMemory{
			{Content: "Test memory", Importance: 0.5},
		},
	}
	resultJSON, _ := json.Marshal(result)
	content := "```json\n" + string(resultJSON) // Missing closing ```

	generate := func(_ context.Context, _ string, _ float64, _ int) (string, error) {
		return content, nil
	}

	got, err := ExtractMemories(context.Background(), generate, "test prompt")
	if err != nil {
		t.Fatalf("ExtractMemories failed: %v", err)
	}
	if len(got.Episodic) != 1 {
		t.Errorf("expected 1 episodic memory, got %d", len(got.Episodic))
	}
}

func TestExtractMemories_InvalidJSON(t *testing.T) {
	generate := func(_ context.Context, _ string, _ float64, _ int) (string, error) {
		return "This is not valid JSON {{{", nil
	}

	_, err := ExtractMemories(context.Background(), generate, "test prompt")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse extraction result") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestAnalyzeMemories_ValidJSON(t *testing.T) {
	result := ConsolidationResult{
		Patterns: []Pattern{
			{Theme: "outdoor", Occurrences: 3, Significance: "high", Insight: "User loves nature"},
		},
		EmotionalArcs: []EmotionalArc{
			{Arc: "anxious to calm", Significance: "medium"},
		},
		RelationshipEvolution: map[string]MetricShift{
			"trust": {Delta: 0.05, Reason: "consistent engagement"},
		},
		ConsolidationSuggestions: []ConsolidationSuggestion{
			{MemoryIDs: []string{"1", "2"}, ConsolidatedContent: "User enjoys hiking and nature"},
		},
	}

	resultJSON, _ := json.Marshal(result)
	generate := func(_ context.Context, _ string, _ float64, _ int) (string, error) {
		return string(resultJSON), nil
	}

	got, err := AnalyzeMemories(context.Background(), generate, "test prompt")
	if err != nil {
		t.Fatalf("AnalyzeMemories failed: %v", err)
	}
	if len(got.Patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(got.Patterns))
	}
	if len(got.ConsolidationSuggestions) != 1 {
		t.Errorf("expected 1 suggestion, got %d", len(got.ConsolidationSuggestions))
	}
}

func TestGeneratePersonalityContext(t *testing.T) {
	generate := func(_ context.Context, _ string, _ float64, _ int) (string, error) {
		return "  You've developed a warm relationship.  ", nil
	}

	got, err := GeneratePersonalityContext(context.Background(), generate, "test prompt")
	if err != nil {
		t.Fatalf("GeneratePersonalityContext failed: %v", err)
	}
	if got != "You've developed a warm relationship." {
		t.Errorf("expected trimmed result, got %q", got)
	}
}
