package mem

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ConversationClient implements ConversationSource via HTTP calls to the chat service.
type ConversationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewConversationClient creates a client pointing at the chat service's internal endpoints.
func NewConversationClient(baseURL string) *ConversationClient {
	return &ConversationClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *ConversationClient) GetMessages(ctx context.Context, conversationID string) ([]ConversationMessage, error) {
	reqURL := fmt.Sprintf("%s/internal/conversations/%s/messages", c.baseURL, conversationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat service request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat service returned %d", resp.StatusCode)
	}

	// The chat service returns chat.Message objects; we only need role, content, created_at.
	var messages []struct {
		Role      string    `json:"role"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
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
	reqURL := fmt.Sprintf("%s/internal/agents/%s", c.baseURL, agentSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat service request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat service returned %d", resp.StatusCode)
	}

	var agent struct {
		Name         string `json:"name"`
		SystemPrompt string `json:"system_prompt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &AgentInfo{
		Name:         agent.Name,
		SystemPrompt: agent.SystemPrompt,
	}, nil
}
