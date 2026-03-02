package mem

import (
	"context"
	"fmt"
	"time"

	"github.com/benaskins/axon"
)

// ConversationClient implements ConversationSource via HTTP calls to the chat service.
type ConversationClient struct {
	client *axon.InternalClient
}

// NewConversationClient creates a client pointing at the chat service's internal endpoints.
func NewConversationClient(baseURL string) *ConversationClient {
	return &ConversationClient{
		client: axon.NewInternalClient(baseURL),
	}
}

func (c *ConversationClient) GetMessages(ctx context.Context, conversationID string) ([]ConversationMessage, error) {
	// The chat service returns chat.Message objects; we only need role, content, created_at.
	var messages []struct {
		Role      string    `json:"role"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}

	path := fmt.Sprintf("/internal/conversations/%s/messages", conversationID)
	if err := c.client.Get(ctx, path, &messages); err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}

	result := make([]ConversationMessage, len(messages))
	for i, m := range messages {
		result[i] = ConversationMessage{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		}
	}
	return result, nil
}

func (c *ConversationClient) GetAgentInfo(ctx context.Context, agentSlug string) (*AgentInfo, error) {
	var agent struct {
		Name         string `json:"name"`
		SystemPrompt string `json:"system_prompt"`
	}

	path := fmt.Sprintf("/internal/agents/%s", agentSlug)
	err := c.client.Get(ctx, path, &agent)
	if err != nil {
		if axon.IsStatusError(err, 404) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("fetch agent: %w", err)
	}

	return &AgentInfo{
		Name:         agent.Name,
		SystemPrompt: agent.SystemPrompt,
	}, nil
}
