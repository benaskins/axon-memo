package memo

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
		Ability:     0.5,
		Benevolence: 0.5,
		Integrity:   0.5,
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
	if !strings.Contains(prompt, "ability=0.50") {
		t.Error("prompt should contain trustworthiness metrics")
	}
}
