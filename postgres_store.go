package memo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const defaultEmbeddingDimension = 1024

const memoSchema = `
CREATE EXTENSION IF NOT EXISTS vector;

CREATE SCHEMA IF NOT EXISTS memory;

CREATE TABLE IF NOT EXISTS memory.memories (
    id               TEXT PRIMARY KEY,
    agent_slug       TEXT NOT NULL,
    user_id          TEXT NOT NULL,
    conversation_id  TEXT,
    memory_type      TEXT NOT NULL,
    content          TEXT NOT NULL,
    emotional_tags   JSONB,
    embedding        vector(%d) NOT NULL,
    importance       DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    consolidated     BOOLEAN NOT NULL DEFAULT false,
    durable          BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_memories_agent_user
    ON memory.memories(agent_slug, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memories_unconsolidated
    ON memory.memories(agent_slug, user_id) WHERE consolidated = false;

CREATE TABLE IF NOT EXISTS memory.extraction_jobs (
    id               TEXT PRIMARY KEY,
    conversation_id  TEXT NOT NULL,
    agent_slug       TEXT NOT NULL,
    user_id          TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending',
    error            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_extraction_jobs_status
    ON memory.extraction_jobs(status) WHERE status IN ('pending', 'processing');

CREATE TABLE IF NOT EXISTS memory.relationship_metrics (
    agent_slug          TEXT NOT NULL,
    user_id             TEXT NOT NULL,
    ability             DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    benevolence         DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    integrity           DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    total_conversations INTEGER NOT NULL DEFAULT 0,
    last_interaction    TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_slug, user_id)
);

CREATE TABLE IF NOT EXISTS memory.personality_contexts (
    agent_slug TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    context    TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_slug, user_id)
);
`

// PostgresStore implements MemoryStore using PostgreSQL with pgvector.
type PostgresStore struct {
	db                 *sql.DB
	embeddingDimension int
}

// PostgresStoreOption configures a PostgresStore.
type PostgresStoreOption func(*PostgresStore)

// WithEmbeddingDimension sets the vector dimension for the embedding column.
// Defaults to 1024 (suitable for nomic-embed-text).
func WithEmbeddingDimension(dim int) PostgresStoreOption {
	return func(s *PostgresStore) {
		s.embeddingDimension = dim
	}
}

// NewPostgresStore opens a connection pool and returns a PostgresStore.
func NewPostgresStore(databaseURL string, opts ...PostgresStoreOption) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	s := &PostgresStore{db: db, embeddingDimension: defaultEmbeddingDimension}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// RunMigrations applies the database schema.
func (s *PostgresStore) RunMigrations(ctx context.Context) error {
	ddl := fmt.Sprintf(memoSchema, s.embeddingDimension)
	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Create the IVFFlat index in a separate statement. The index requires
	// rows to already exist for IVFFlat training, but CREATE INDEX IF NOT EXISTS
	// is idempotent, so we attempt it on every migration run.
	ivfflat := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_memories_embedding
		    ON memory.memories
		    USING ivfflat (embedding vector_cosine_ops)
		    WITH (lists = 100)
	`)
	if _, err := s.db.ExecContext(ctx, ivfflat); err != nil {
		// IVFFlat may fail on empty tables — log but do not block startup.
		// The exact-scan fallback works until enough rows exist to build the index.
		_ = err
	}

	return nil
}

// Close closes the underlying database connection pool.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// --- MemoryStore implementation ---

func (s *PostgresStore) SaveMemory(ctx context.Context, mem Memory) (string, error) {
	if mem.ID == "" {
		mem.ID = newUUID()
	}
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now()
	}

	var emotionalJSON []byte
	if mem.EmotionalTags != nil {
		var err error
		emotionalJSON, err = json.Marshal(mem.EmotionalTags)
		if err != nil {
			return "", fmt.Errorf("marshal emotional tags: %w", err)
		}
	}

	embeddingStr := vectorLiteral(mem.Embedding)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory.memories
			(id, agent_slug, user_id, conversation_id, memory_type, content, emotional_tags, embedding, importance, created_at, consolidated, durable)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::vector, $9, $10, $11, $12)
	`, mem.ID, mem.AgentSlug, mem.UserID, mem.ConversationID, mem.MemoryType,
		mem.Content, emotionalJSON, embeddingStr, mem.Importance, mem.CreatedAt,
		mem.Consolidated, mem.Durable)
	if err != nil {
		return "", fmt.Errorf("insert memory: %w", err)
	}
	return mem.ID, nil
}

func (s *PostgresStore) SearchMemoriesByVector(ctx context.Context, agentSlug, userID string, embedding []float64, limit int) ([]MemoryWithDistance, error) {
	embeddingStr := vectorLiteral(embedding)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_slug, user_id, conversation_id, memory_type, content,
		       emotional_tags, importance, created_at, consolidated, durable,
		       embedding <=> $1::vector AS distance
		FROM memory.memories
		WHERE agent_slug = $2 AND user_id = $3
		ORDER BY embedding <=> $1::vector
		LIMIT $4
	`, embeddingStr, agentSlug, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var results []MemoryWithDistance
	for rows.Next() {
		var mwd MemoryWithDistance
		var emotionalJSON []byte
		var conversationID sql.NullString
		if err := rows.Scan(
			&mwd.ID, &mwd.AgentSlug, &mwd.UserID, &conversationID,
			&mwd.MemoryType, &mwd.Content, &emotionalJSON,
			&mwd.Importance, &mwd.CreatedAt, &mwd.Consolidated, &mwd.Durable,
			&mwd.Distance,
		); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if conversationID.Valid {
			mwd.ConversationID = &conversationID.String
		}
		if len(emotionalJSON) > 0 {
			var tags EmotionalTags
			if err := json.Unmarshal(emotionalJSON, &tags); err == nil {
				mwd.EmotionalTags = &tags
			}
		}
		results = append(results, mwd)
	}
	return results, rows.Err()
}

func (s *PostgresStore) CreateExtractionJob(ctx context.Context, conversationID, agentSlug, userID string) (*ExtractionJob, error) {
	job := &ExtractionJob{
		ID:             newUUID(),
		ConversationID: conversationID,
		AgentSlug:      agentSlug,
		UserID:         userID,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory.extraction_jobs (id, conversation_id, agent_slug, user_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, job.ID, job.ConversationID, job.AgentSlug, job.UserID, job.Status, job.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert extraction job: %w", err)
	}
	return job, nil
}

func (s *PostgresStore) UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE memory.extraction_jobs
		SET status = $2, error = $3, completed_at = $4
		WHERE id = $1
	`, jobID, status, errorMsg, completedAt)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetUnconsolidatedMemories(ctx context.Context, agentSlug, userID string) ([]Memory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_slug, user_id, conversation_id, memory_type, content,
		       emotional_tags, importance, created_at, consolidated, durable
		FROM memory.memories
		WHERE agent_slug = $1 AND user_id = $2 AND consolidated = false
		ORDER BY created_at
	`, agentSlug, userID)
	if err != nil {
		return nil, fmt.Errorf("get unconsolidated memories: %w", err)
	}
	defer rows.Close()

	return scanMemories(rows)
}

func (s *PostgresStore) MarkMemoriesConsolidated(ctx context.Context, memoryIDs []string) error {
	if len(memoryIDs) == 0 {
		return nil
	}

	// Build parameterized IN clause.
	placeholders := make([]string, len(memoryIDs))
	args := make([]interface{}, len(memoryIDs))
	for i, id := range memoryIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		UPDATE memory.memories SET consolidated = true
		WHERE id IN (%s)
	`, strings.Join(placeholders, ", "))

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark memories consolidated: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetOrCreateRelationshipMetrics(ctx context.Context, agentSlug, userID string) (*RelationshipMetrics, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory.relationship_metrics (agent_slug, user_id)
		VALUES ($1, $2)
		ON CONFLICT (agent_slug, user_id) DO NOTHING
	`, agentSlug, userID)
	if err != nil {
		return nil, fmt.Errorf("upsert relationship metrics: %w", err)
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT agent_slug, user_id, ability, benevolence, integrity,
		       total_conversations, last_interaction, updated_at
		FROM memory.relationship_metrics
		WHERE agent_slug = $1 AND user_id = $2
	`, agentSlug, userID)

	var m RelationshipMetrics
	if err := row.Scan(
		&m.AgentSlug, &m.UserID, &m.Ability, &m.Benevolence, &m.Integrity,
		&m.TotalConversations, &m.LastInteraction, &m.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan relationship metrics: %w", err)
	}
	return &m, nil
}

func (s *PostgresStore) UpdateRelationshipMetrics(ctx context.Context, agentSlug, userID string, shifts map[string]float64) error {
	setClauses := []string{"updated_at = now()"}
	args := []interface{}{agentSlug, userID}
	idx := 3

	for _, col := range []string{"ability", "benevolence", "integrity"} {
		if delta, ok := shifts[col]; ok {
			setClauses = append(setClauses,
				fmt.Sprintf("%s = LEAST(1, GREATEST(0, %s + $%d))", col, col, idx))
			args = append(args, delta)
			idx++
		}
	}

	if _, ok := shifts["total_conversations"]; ok {
		setClauses = append(setClauses,
			fmt.Sprintf("total_conversations = total_conversations + $%d", idx))
		args = append(args, int(shifts["total_conversations"]))
		idx++
	}

	query := fmt.Sprintf(`
		UPDATE memory.relationship_metrics
		SET %s
		WHERE agent_slug = $1 AND user_id = $2
	`, strings.Join(setClauses, ", "))

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update relationship metrics: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SavePersonalityContext(ctx context.Context, agentSlug, userID, personalityContext string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory.personality_contexts (agent_slug, user_id, context, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (agent_slug, user_id) DO UPDATE
		SET context = EXCLUDED.context, updated_at = now()
	`, agentSlug, userID, personalityContext)
	if err != nil {
		return fmt.Errorf("save personality context: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetPersonalityContext(ctx context.Context, agentSlug, userID string) (string, error) {
	var ctx_ string
	err := s.db.QueryRowContext(ctx, `
		SELECT context FROM memory.personality_contexts
		WHERE agent_slug = $1 AND user_id = $2
	`, agentSlug, userID).Scan(&ctx_)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get personality context: %w", err)
	}
	return ctx_, nil
}

func (s *PostgresStore) GetActiveAgents(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT agent_slug
		FROM memory.memories
		WHERE consolidated = false
	`)
	if err != nil {
		return nil, fmt.Errorf("get active agents: %w", err)
	}
	defer rows.Close()

	var agents []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		agents = append(agents, slug)
	}
	return agents, rows.Err()
}

func (s *PostgresStore) GetActiveUsers(ctx context.Context, agentSlug string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT user_id
		FROM memory.memories
		WHERE agent_slug = $1 AND consolidated = false
	`, agentSlug)
	if err != nil {
		return nil, fmt.Errorf("get active users: %w", err)
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		users = append(users, id)
	}
	return users, rows.Err()
}

// --- helpers ---

// scanMemories reads Memory rows from a *sql.Rows. The caller must close rows.
func scanMemories(rows *sql.Rows) ([]Memory, error) {
	var memories []Memory
	for rows.Next() {
		var m Memory
		var emotionalJSON []byte
		var conversationID sql.NullString
		if err := rows.Scan(
			&m.ID, &m.AgentSlug, &m.UserID, &conversationID,
			&m.MemoryType, &m.Content, &emotionalJSON,
			&m.Importance, &m.CreatedAt, &m.Consolidated, &m.Durable,
		); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if conversationID.Valid {
			m.ConversationID = &conversationID.String
		}
		if len(emotionalJSON) > 0 {
			var tags EmotionalTags
			if err := json.Unmarshal(emotionalJSON, &tags); err == nil {
				m.EmotionalTags = &tags
			}
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// vectorLiteral formats a float64 slice as a pgvector literal, e.g. "[0.1,0.2,0.3]".
func vectorLiteral(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// newUUID generates a random UUID v4.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = cryptoRandRead(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// cryptoRandRead wraps crypto/rand.Read for UUID generation.
var cryptoRandRead = rand.Read
