package memo

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/benaskins/axon"
	fact "github.com/benaskins/axon-fact"
)

// Server provides HTTP handlers for the memory service.
type Server struct {
	store        MemoryStore
	extractor    *Extractor
	retriever    *Retriever
	consolidator *Consolidator
	scheduler    *Scheduler

	analytics  AnalyticsEmitter
	eventStore fact.EventStore
}

// NewServer creates a Server with all memory service components.
func NewServer(store MemoryStore, extractor *Extractor, retriever *Retriever, consolidator *Consolidator, opts ...Option) *Server {
	s := &Server{
		store:        store,
		extractor:    extractor,
		retriever:    retriever,
		consolidator: consolidator,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Handler returns an http.Handler with all memory service routes.
func (s *Server) Handler() http.Handler {
	if s.analytics != nil {
		s.extractor.analytics = s.analytics
		s.consolidator.analytics = s.analytics
	}
	if s.eventStore != nil {
		s.extractor.eventStore = s.eventStore
		s.consolidator.eventStore = s.eventStore
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/memory/store", s.handleStore)
	mux.HandleFunc("POST /api/memory/extract", s.handleExtract)
	mux.HandleFunc("GET /api/memory/recall", s.handleRecall)
	mux.HandleFunc("POST /api/memory/consolidate", s.handleConsolidate)
	return mux
}

// StartScheduler begins the 2AM daily consolidation schedule.
func (s *Server) StartScheduler() {
	s.scheduler = NewScheduler(s.consolidator)
	s.scheduler.Start()
}

// StopScheduler stops the consolidation scheduler.
func (s *Server) StopScheduler() {
	if s.scheduler != nil {
		s.scheduler.Stop()
	}
}

func (s *Server) handleStore(w http.ResponseWriter, r *http.Request) {
	req, ok := axon.DecodeJSON[StoreRequest](w, r)
	if !ok {
		return
	}

	if req.AgentSlug == "" || req.UserID == "" || req.Content == "" {
		axon.WriteError(w, http.StatusBadRequest, "Missing required fields: agent_slug, user_id, content")
		return
	}

	if req.MemoryType == "" {
		req.MemoryType = "semantic"
	}

	if req.Importance == 0 {
		req.Importance = 0.5
	}

	ctx := r.Context()

	embedding, err := s.retriever.embed(ctx, req.Content)
	if err != nil {
		slog.Error("failed to generate embedding", "error", err)
		axon.WriteError(w, http.StatusInternalServerError, "Failed to generate embedding")
		return
	}

	id, err := s.store.SaveMemory(ctx, Memory{
		AgentSlug:  req.AgentSlug,
		UserID:     req.UserID,
		MemoryType: req.MemoryType,
		Content:    req.Content,
		Embedding:  embedding,
		Importance: req.Importance,
		Durable:    req.Durable,
	})
	if err != nil {
		slog.Error("failed to store memory", "error", err)
		axon.WriteError(w, http.StatusInternalServerError, "Failed to store memory")
		return
	}

	axon.WriteJSON(w, http.StatusCreated, StoreResponse{
		ID:      id,
		Status:  "stored",
		Durable: req.Durable,
	})
}

func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	req, ok := axon.DecodeJSON[ExtractRequest](w, r)
	if !ok {
		return
	}

	if req.ConversationID == "" || req.AgentSlug == "" || req.UserID == "" {
		axon.WriteError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	ctx := r.Context()

	job, err := s.store.CreateExtractionJob(ctx, req.ConversationID, req.AgentSlug, req.UserID)
	if err != nil {
		slog.Error("failed to create extraction job", "error", err)
		axon.WriteError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}

	go func(ctx context.Context) {
		if err := s.extractor.ExtractConversation(ctx, job.ID, req.ConversationID, req.AgentSlug, req.UserID); err != nil {
			slog.Error("extraction failed", "job_id", job.ID, "error", err)
		}
	}(context.WithoutCancel(r.Context()))

	axon.WriteJSON(w, http.StatusAccepted, ExtractResponse{
		JobID:     job.ID,
		Status:    "pending",
		CreatedAt: job.CreatedAt,
	})
}

func (s *Server) handleRecall(w http.ResponseWriter, r *http.Request) {
	agentSlug := r.URL.Query().Get("agent")
	userID := r.URL.Query().Get("user")
	query := r.URL.Query().Get("query")
	limitStr := r.URL.Query().Get("limit")

	if agentSlug == "" || userID == "" || query == "" {
		axon.WriteError(w, http.StatusBadRequest, "Missing required parameters: agent, user, query")
		return
	}

	limit := 5
	if limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
			axon.WriteError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	ctx := r.Context()

	response, err := s.retriever.Recall(ctx, RecallRequest{
		AgentSlug: agentSlug,
		UserID:    userID,
		Query:     query,
		Limit:     limit,
	})

	if err != nil {
		slog.Error("recall failed", "error", err, "agent", agentSlug, "user", userID)
		axon.WriteError(w, http.StatusInternalServerError, "Recall failed")
		return
	}

	axon.WriteJSON(w, http.StatusOK, response)
}

func (s *Server) handleConsolidate(w http.ResponseWriter, r *http.Request) {
	req, ok := axon.DecodeJSON[ConsolidateRequest](w, r)
	if !ok {
		return
	}

	if req.AgentSlug == "" || req.UserID == "" {
		axon.WriteError(w, http.StatusBadRequest, "Missing required fields: agent_slug, user_id")
		return
	}

	ctx := r.Context()

	if err := s.consolidator.ConsolidateAgent(ctx, req.AgentSlug, req.UserID); err != nil {
		slog.Error("consolidation failed", "error", err, "agent", req.AgentSlug, "user", req.UserID)
		axon.WriteError(w, http.StatusInternalServerError, "Consolidation failed")
		return
	}

	axon.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "completed",
		"agent":  req.AgentSlug,
		"user":   req.UserID,
	})
}
