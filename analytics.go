package memo

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// AnalyticsEvent is a typed event sent to the analytics service.
type AnalyticsEvent struct {
	Type           string    `json:"type"`
	Timestamp      time.Time `json:"timestamp"`
	AgentSlug      string    `json:"agent_slug,omitempty"`
	UserID         string    `json:"user_id,omitempty"`
	RunID          string    `json:"run_id,omitempty"`
	MemoryType     string    `json:"memory_type,omitempty"`
	Importance     float32   `json:"importance,omitempty"`
	Ability        float32   `json:"ability,omitempty"`
	Benevolence    float32   `json:"benevolence,omitempty"`
	Integrity      float32   `json:"integrity,omitempty"`
	PatternsFound  uint16    `json:"patterns_found,omitempty"`
	MemoriesMerged uint16    `json:"memories_merged,omitempty"`
}

// AnalyticsEmitter sends analytics events.
type AnalyticsEmitter interface {
	Emit(events ...AnalyticsEvent)
}

// AnalyticsClient sends events to the analytics service over HTTP.
type AnalyticsClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAnalyticsClient creates a client for the analytics service.
func NewAnalyticsClient(baseURL string) *AnalyticsClient {
	return &AnalyticsClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Emit sends events asynchronously.
func (c *AnalyticsClient) Emit(events ...AnalyticsEvent) {
	go func() {
		body, err := json.Marshal(events)
		if err != nil {
			slog.Error("analytics: failed to marshal events", "error", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/events", bytes.NewReader(body))
		if err != nil {
			slog.Error("analytics: failed to create request", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			slog.Warn("analytics: failed to send events", "error", err)
			return
		}
		resp.Body.Close()
	}()
}

// NoopAnalytics discards all events.
type NoopAnalytics struct{}

func (NoopAnalytics) Emit(events ...AnalyticsEvent) {}

// MemoryExtractedEvent creates a memory_extracted analytics event.
func MemoryExtractedEvent(agentSlug, userID, memoryType string, importance float64) AnalyticsEvent {
	return AnalyticsEvent{
		Type:       "memory_extracted",
		Timestamp:  time.Now(),
		AgentSlug:  agentSlug,
		UserID:     userID,
		MemoryType: memoryType,
		Importance: float32(importance),
	}
}

// RelationshipSnapshotEvent creates a relationship_snapshot analytics event.
func RelationshipSnapshotEvent(agentSlug, userID string, metrics *RelationshipMetrics) AnalyticsEvent {
	return AnalyticsEvent{
		Type:        "relationship_snapshot",
		Timestamp:   time.Now(),
		AgentSlug:   agentSlug,
		UserID:      userID,
		Ability:     float32(metrics.Ability),
		Benevolence: float32(metrics.Benevolence),
		Integrity:   float32(metrics.Integrity),
	}
}

// ConsolidationCompletedEvent creates a consolidation_completed analytics event.
func ConsolidationCompletedEvent(agentSlug, userID string, patterns, merged int) AnalyticsEvent {
	return AnalyticsEvent{
		Type:           "consolidation_completed",
		Timestamp:      time.Now(),
		AgentSlug:      agentSlug,
		UserID:         userID,
		PatternsFound:  uint16(patterns),
		MemoriesMerged: uint16(merged),
	}
}
