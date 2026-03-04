package memo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAnalyticsClient_Emit(t *testing.T) {
	var mu sync.Mutex
	var received []AnalyticsEvent

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var events []AnalyticsEvent
		json.NewDecoder(r.Body).Decode(&events)
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewAnalyticsClient(server.URL)
	client.Emit(
		MemoryExtractedEvent("helper", "user1", "episodic", 0.8),
		MemoryExtractedEvent("helper", "user1", "semantic", 0.6),
	)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if received[0].Type != "memory_extracted" {
		t.Errorf("expected memory_extracted, got %s", received[0].Type)
	}
	if received[0].MemoryType != "episodic" {
		t.Errorf("expected episodic, got %s", received[0].MemoryType)
	}
}

func TestMemoryExtractedEvent(t *testing.T) {
	e := MemoryExtractedEvent("bot", "u1", "emotional", 0.9)
	if e.Type != "memory_extracted" {
		t.Errorf("expected memory_extracted, got %s", e.Type)
	}
	if e.MemoryType != "emotional" {
		t.Errorf("expected emotional, got %s", e.MemoryType)
	}
}

func TestRelationshipSnapshotEvent(t *testing.T) {
	metrics := &RelationshipMetrics{
		Trust:   0.8,
		Intimacy: 0.6,
		Autonomy: 0.5,
	}
	e := RelationshipSnapshotEvent("bot", "u1", metrics)
	if e.Type != "relationship_snapshot" {
		t.Errorf("expected relationship_snapshot, got %s", e.Type)
	}
	if e.Trust != 0.8 {
		t.Errorf("expected trust 0.8, got %f", e.Trust)
	}
}

func TestConsolidationCompletedEvent(t *testing.T) {
	e := ConsolidationCompletedEvent("bot", "u1", 3, 5)
	if e.Type != "consolidation_completed" {
		t.Errorf("expected consolidation_completed, got %s", e.Type)
	}
	if e.PatternsFound != 3 {
		t.Errorf("expected 3 patterns, got %d", e.PatternsFound)
	}
	if e.MemoriesMerged != 5 {
		t.Errorf("expected 5 merged, got %d", e.MemoriesMerged)
	}
}
