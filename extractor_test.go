package mem

import (
	"strings"
	"testing"
)

func TestBuildExtractionPrompt(t *testing.T) {
	messages := []ConversationMessage{
		{Role: "user", Content: "Hello!"},
		{Role: "assistant", Content: "Hi there!"},
	}

	agent := &AgentInfo{
		Name:         "Test Agent",
		SystemPrompt: "A helpful assistant",
	}

	metrics := &RelationshipMetrics{
		Trust:       0.5,
		Intimacy:    0.5,
		Autonomy:    0.5,
		Reciprocity: 0.5,
		Playfulness: 0.5,
		Conflict:    0.0,
	}

	prompt := BuildExtractionPrompt(messages, agent, metrics)

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}

	if !strings.Contains(prompt, "Test Agent") {
		t.Error("prompt should contain agent name")
	}
	if !strings.Contains(prompt, "Hello!") {
		t.Error("prompt should contain conversation content")
	}
	if !strings.Contains(prompt, "trust=0.50") {
		t.Error("prompt should contain relationship metrics")
	}
}
