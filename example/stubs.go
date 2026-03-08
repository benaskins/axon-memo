package main

import (
	"context"
	"time"

	memo "github.com/benaskins/axon-memo"
)

// stubConversationSource returns empty data. Replace with
// memo.NewConversationClient(chatServiceURL) in production.
type stubConversationSource struct{}

func (s *stubConversationSource) GetMessages(_ context.Context, conversationID string) ([]memo.ConversationMessage, error) {
	return []memo.ConversationMessage{
		{Role: "user", Content: "Hello, how are you?", CreatedAt: time.Now()},
		{Role: "assistant", Content: "I'm doing well, thanks for asking!", CreatedAt: time.Now()},
	}, nil
}

func (s *stubConversationSource) GetAgentInfo(_ context.Context, agentSlug string) (*memo.AgentInfo, error) {
	return &memo.AgentInfo{
		Name:         agentSlug,
		SystemPrompt: "You are a helpful assistant.",
	}, nil
}

// stubTextGenerator returns a placeholder response. Replace with a real
// LLM client (Ollama, Claude API, etc.) in production.
func stubTextGenerator(_ context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
	return `{"episodic": [], "semantic": [], "emotional": [], "relationship_shifts": {}}`, nil
}

// stubEmbeddingGenerator returns a zero vector. Replace with a real
// embedding model (e.g. Ollama nomic-embed-text) in production.
func stubEmbeddingGenerator(_ context.Context, text string) ([]float64, error) {
	// Return a small dummy vector — real models produce 768-1024 dimensions
	vec := make([]float64, 64)
	for i, c := range text {
		if i >= len(vec) {
			break
		}
		vec[i] = float64(c) / 256.0
	}
	return vec, nil
}
