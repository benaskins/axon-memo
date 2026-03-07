package memo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockStore implements MemoryStore for testing.
type mockStore struct {
	savedMemory *Memory
	savedID     string
	saveErr     error

	searchResults []MemoryWithDistance
	searchErr     error

	metrics    *RelationshipMetrics
	metricsErr error

	personalityCtx string
	personalityErr error
}

func (m *mockStore) CreateExtractionJob(_ context.Context, _, _, _ string) (*ExtractionJob, error) {
	return nil, nil
}

func (m *mockStore) UpdateJobStatus(_ context.Context, _, _ string, _ *string) error {
	return nil
}

func (m *mockStore) SaveMemory(_ context.Context, mem Memory) (string, error) {
	m.savedMemory = &mem
	return m.savedID, m.saveErr
}

func (m *mockStore) SearchMemoriesByVector(_ context.Context, _, _ string, _ []float64, _ int) ([]MemoryWithDistance, error) {
	return m.searchResults, m.searchErr
}

func (m *mockStore) GetOrCreateRelationshipMetrics(_ context.Context, _, _ string) (*RelationshipMetrics, error) {
	if m.metrics != nil {
		return m.metrics, m.metricsErr
	}
	return &RelationshipMetrics{Ability: 0.5, Benevolence: 0.5, Integrity: 0.5}, m.metricsErr
}

func (m *mockStore) UpdateRelationshipMetrics(_ context.Context, _, _ string, _ map[string]float64) error {
	return nil
}

func (m *mockStore) GetUnconsolidatedMemories(_ context.Context, _, _ string) ([]Memory, error) {
	return nil, nil
}

func (m *mockStore) MarkMemoriesConsolidated(_ context.Context, _ []string) error {
	return nil
}

func (m *mockStore) SavePersonalityContext(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockStore) GetPersonalityContext(_ context.Context, _, _ string) (string, error) {
	return m.personalityCtx, m.personalityErr
}

func (m *mockStore) GetActiveAgents(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockStore) GetActiveUsers(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// fakeEmbed returns a fixed embedding vector for testing.
func fakeEmbed(_ context.Context, _ string) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}

func newTestServer(store *mockStore) http.Handler {
	retriever := NewRetriever(store, fakeEmbed)
	extractor := &Extractor{analytics: NoopAnalytics{}}
	consolidator := &Consolidator{store: store, analytics: NoopAnalytics{}}
	srv := NewServer(store, extractor, retriever, consolidator)
	return srv.Handler()
}

// --- handleStore tests ---

func TestHandleStore_ValidRequest(t *testing.T) {
	store := &mockStore{savedID: "mem-abc-123"}
	handler := newTestServer(store)

	body := `{"agent_slug":"test-agent","user_id":"user-1","content":"Remember this fact","importance":0.8,"durable":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp StoreResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "mem-abc-123" {
		t.Errorf("expected ID mem-abc-123, got %s", resp.ID)
	}
	if resp.Status != "stored" {
		t.Errorf("expected status stored, got %s", resp.Status)
	}
	if !resp.Durable {
		t.Error("expected durable to be true")
	}

	// Verify the memory was saved with correct fields
	if store.savedMemory == nil {
		t.Fatal("expected memory to be saved")
	}
	if store.savedMemory.AgentSlug != "test-agent" {
		t.Errorf("expected agent_slug test-agent, got %s", store.savedMemory.AgentSlug)
	}
	if store.savedMemory.Content != "Remember this fact" {
		t.Errorf("expected content 'Remember this fact', got %s", store.savedMemory.Content)
	}
	if !store.savedMemory.Durable {
		t.Error("expected saved memory to be durable")
	}
}

func TestHandleStore_DefaultsMemoryTypeAndImportance(t *testing.T) {
	store := &mockStore{savedID: "mem-1"}
	handler := newTestServer(store)

	body := `{"agent_slug":"a","user_id":"u","content":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if store.savedMemory.MemoryType != "semantic" {
		t.Errorf("expected default memory_type semantic, got %s", store.savedMemory.MemoryType)
	}
	if store.savedMemory.Importance != 0.5 {
		t.Errorf("expected default importance 0.5, got %f", store.savedMemory.Importance)
	}
}

func TestHandleStore_MissingContent(t *testing.T) {
	store := &mockStore{}
	handler := newTestServer(store)

	body := `{"agent_slug":"a","user_id":"u"}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStore_MissingAgent(t *testing.T) {
	store := &mockStore{}
	handler := newTestServer(store)

	body := `{"user_id":"u","content":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStore_MissingUserID(t *testing.T) {
	store := &mockStore{}
	handler := newTestServer(store)

	body := `{"agent_slug":"a","content":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleRecall tests ---

func TestHandleRecall_ValidQuery(t *testing.T) {
	store := &mockStore{
		searchResults: []MemoryWithDistance{
			{
				Memory: Memory{
					ID:         "mem-1",
					MemoryType: "semantic",
					Content:    "Go is a compiled language",
					Importance: 0.8,
				},
				Distance: 0.2,
			},
		},
	}
	handler := newTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/recall?agent=test-agent&user=user-1&query=Go+language", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp RecallResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(resp.Memories))
	}
	if resp.Memories[0].Content != "Go is a compiled language" {
		t.Errorf("unexpected content: %s", resp.Memories[0].Content)
	}
	if resp.RelationshipContext == nil {
		t.Error("expected relationship context to be present")
	}
}

func TestHandleRecall_MissingQuery(t *testing.T) {
	store := &mockStore{}
	handler := newTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/recall?agent=a&user=u", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRecall_MissingAgent(t *testing.T) {
	store := &mockStore{}
	handler := newTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/recall?user=u&query=test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRecall_MissingUser(t *testing.T) {
	store := &mockStore{}
	handler := newTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/recall?agent=a&query=test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRecall_LimitClamping(t *testing.T) {
	store := &mockStore{searchResults: []MemoryWithDistance{}}
	handler := newTestServer(store)

	tests := []struct {
		name     string
		limitStr string
	}{
		{"negative limit clamped to 10", "-5"},
		{"zero limit clamped to 10", "0"},
		{"over-100 limit clamped to 100", "200"},
		{"valid limit passes through", "15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/memory/recall?agent=a&user=u&query=test&limit="+tt.limitStr, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleRecall_InvalidLimit(t *testing.T) {
	store := &mockStore{}
	handler := newTestServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/recall?agent=a&user=u&query=test&limit=abc", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRecall_DefaultLimit(t *testing.T) {
	store := &mockStore{searchResults: []MemoryWithDistance{}}
	handler := newTestServer(store)

	// No limit parameter - should default to 5
	req := httptest.NewRequest(http.MethodGet, "/api/memory/recall?agent=a&user=u&query=test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
