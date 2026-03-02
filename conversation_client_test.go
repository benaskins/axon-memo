package mem

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConversationClient_GetMessages(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	messages := []map[string]any{
		{"id": "m1", "conversation_id": "c1", "role": "user", "content": "hello", "created_at": now},
		{"id": "m2", "conversation_id": "c1", "role": "assistant", "content": "hi", "created_at": now},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/conversations/c1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	}))
	defer srv.Close()

	client := NewConversationClient(srv.URL)
	msgs, err := client.GetMessages(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi" {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestConversationClient_GetMessages_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewConversationClient(srv.URL)
	_, err := client.GetMessages(context.Background(), "c1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConversationClient_GetAgentInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/agents/bot" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"name":          "Bot",
			"system_prompt": "You are a helpful bot.",
		})
	}))
	defer srv.Close()

	client := NewConversationClient(srv.URL)
	info, err := client.GetAgentInfo(context.Background(), "bot")
	if err != nil {
		t.Fatalf("GetAgentInfo: %v", err)
	}
	if info.Name != "Bot" {
		t.Errorf("expected name Bot, got %s", info.Name)
	}
	if info.SystemPrompt != "You are a helpful bot." {
		t.Errorf("expected system prompt, got %s", info.SystemPrompt)
	}
}

func TestConversationClient_GetAgentInfo_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "agent not found"})
	}))
	defer srv.Close()

	client := NewConversationClient(srv.URL)
	_, err := client.GetAgentInfo(context.Background(), "missing")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
