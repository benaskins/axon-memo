package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	memo "github.com/benaskins/axon-memo"
)

// InMemoryStore implements memo.MemoryStore for development and testing.
// Not suitable for production — no persistence, no real vector indexing.
type InMemoryStore struct {
	mu           sync.Mutex
	memories     []memo.Memory
	jobs         map[string]*memo.ExtractionJob
	metrics      map[string]*memo.RelationshipMetrics // key: "agent:user"
	personalities map[string]string                    // key: "agent:user"
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		jobs:          make(map[string]*memo.ExtractionJob),
		metrics:       make(map[string]*memo.RelationshipMetrics),
		personalities: make(map[string]string),
	}
}

func (s *InMemoryStore) SaveMemory(_ context.Context, mem memo.Memory) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mem.ID = newID()
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now()
	}
	s.memories = append(s.memories, mem)
	return mem.ID, nil
}

func (s *InMemoryStore) SearchMemoriesByVector(_ context.Context, agentSlug, userID string, embedding []float64, limit int) ([]memo.MemoryWithDistance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type scored struct {
		mem      memo.Memory
		distance float64
	}

	var candidates []scored
	for _, m := range s.memories {
		if m.AgentSlug != agentSlug || m.UserID != userID {
			continue
		}
		d := cosineDistance(embedding, m.Embedding)
		candidates = append(candidates, scored{mem: m, distance: d})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	if limit > len(candidates) {
		limit = len(candidates)
	}

	results := make([]memo.MemoryWithDistance, limit)
	for i := 0; i < limit; i++ {
		results[i] = memo.MemoryWithDistance{
			Memory:   candidates[i].mem,
			Distance: candidates[i].distance,
		}
	}
	return results, nil
}

func (s *InMemoryStore) CreateExtractionJob(_ context.Context, conversationID, agentSlug, userID string) (*memo.ExtractionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := &memo.ExtractionJob{
		ID:             newID(),
		ConversationID: conversationID,
		AgentSlug:      agentSlug,
		UserID:         userID,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	s.jobs[job.ID] = job
	return job, nil
}

func (s *InMemoryStore) UpdateJobStatus(_ context.Context, jobID, status string, errorMsg *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return memo.ErrNotFound
	}
	job.Status = status
	job.Error = errorMsg
	if status == "completed" || status == "failed" {
		now := time.Now()
		job.CompletedAt = &now
	}
	return nil
}

func (s *InMemoryStore) GetOrCreateRelationshipMetrics(_ context.Context, agentSlug, userID string) (*memo.RelationshipMetrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := agentSlug + ":" + userID
	m, ok := s.metrics[key]
	if !ok {
		m = &memo.RelationshipMetrics{
			AgentSlug:   agentSlug,
			UserID:      userID,
			Ability:     0.5,
			Benevolence: 0.5,
			Integrity:   0.5,
			UpdatedAt:   time.Now(),
		}
		s.metrics[key] = m
	}
	return m, nil
}

func (s *InMemoryStore) UpdateRelationshipMetrics(_ context.Context, agentSlug, userID string, shifts map[string]float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := agentSlug + ":" + userID
	m, ok := s.metrics[key]
	if !ok {
		m = &memo.RelationshipMetrics{
			AgentSlug:   agentSlug,
			UserID:      userID,
			Ability:     0.5,
			Benevolence: 0.5,
			Integrity:   0.5,
		}
		s.metrics[key] = m
	}

	if d, ok := shifts["ability"]; ok {
		m.Ability = clamp(m.Ability+d, 0, 1)
	}
	if d, ok := shifts["benevolence"]; ok {
		m.Benevolence = clamp(m.Benevolence+d, 0, 1)
	}
	if d, ok := shifts["integrity"]; ok {
		m.Integrity = clamp(m.Integrity+d, 0, 1)
	}
	m.UpdatedAt = time.Now()
	return nil
}

func (s *InMemoryStore) GetUnconsolidatedMemories(_ context.Context, agentSlug, userID string) ([]memo.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []memo.Memory
	for _, m := range s.memories {
		if m.AgentSlug == agentSlug && m.UserID == userID && !m.Consolidated {
			result = append(result, m)
		}
	}
	return result, nil
}

func (s *InMemoryStore) MarkMemoriesConsolidated(_ context.Context, memoryIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make(map[string]bool, len(memoryIDs))
	for _, id := range memoryIDs {
		ids[id] = true
	}
	for i := range s.memories {
		if ids[s.memories[i].ID] {
			s.memories[i].Consolidated = true
		}
	}
	return nil
}

func (s *InMemoryStore) SavePersonalityContext(_ context.Context, agentSlug, userID, ctx string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.personalities[agentSlug+":"+userID] = ctx
	return nil
}

func (s *InMemoryStore) GetPersonalityContext(_ context.Context, agentSlug, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.personalities[agentSlug+":"+userID], nil
}

func (s *InMemoryStore) GetActiveAgents(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := map[string]bool{}
	for _, m := range s.memories {
		if !m.Consolidated {
			seen[m.AgentSlug] = true
		}
	}
	agents := make([]string, 0, len(seen))
	for a := range seen {
		agents = append(agents, a)
	}
	return agents, nil
}

func (s *InMemoryStore) GetActiveUsers(_ context.Context, agentSlug string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := map[string]bool{}
	for _, m := range s.memories {
		if m.AgentSlug == agentSlug && !m.Consolidated {
			seen[m.UserID] = true
		}
	}
	users := make([]string, 0, len(seen))
	for u := range seen {
		users = append(users, u)
	}
	return users, nil
}

// cosineDistance computes 1 - cosine_similarity between two vectors.
func cosineDistance(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	return 1.0 - (dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
