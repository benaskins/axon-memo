package memo

import (
	"context"
	"encoding/json"
	"fmt"
	"crypto/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

// schemaSQL returns CREATE statements for all memo tables.
// dim is the embedding vector dimension (e.g. 1024 for nomic-embed-text).
func schemaSQL(dim int) string {
	return fmt.Sprintf(`
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS memories (
    id              TEXT PRIMARY KEY,
    agent_slug      TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    conversation_id TEXT,
    memory_type     TEXT NOT NULL,
    content         TEXT NOT NULL,
    emotional_tags  JSONB,
    embedding       vector(%d) NOT NULL,
    importance      FLOAT8 NOT NULL DEFAULT 0.5,
    created_at      TIMESTAMPTZ NOT NULL,
    consolidated    BOOL NOT NULL DEFAULT FALSE,
    durable         BOOL NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_memories_agent
    ON memories (agent_slug, user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memories_unconsolidated
    ON memories (agent_slug, user_id) WHERE consolidated = FALSE;

CREATE TABLE IF NOT EXISTS extraction_jobs (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    agent_slug      TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS relationship_metrics (
    agent_slug           TEXT NOT NULL,
    user_id              TEXT NOT NULL,
    ability              FLOAT8 NOT NULL DEFAULT 0.5,
    benevolence          FLOAT8 NOT NULL DEFAULT 0.5,
    integrity            FLOAT8 NOT NULL DEFAULT 0.5,
    total_conversations  INT NOT NULL DEFAULT 0,
    last_interaction     TIMESTAMPTZ,
    updated_at           TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (agent_slug, user_id)
);

CREATE TABLE IF NOT EXISTS personality_contexts (
    agent_slug  TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    context     TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (agent_slug, user_id)
);
`, dim)
}

// indexSQL is the IVFFlat ANN index — run after enough rows exist (>= 3*lists = 300).
const indexSQL = `
CREATE INDEX IF NOT EXISTS idx_memories_vector
    ON memories USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
`

// PostgresStore implements MemoryStore using PostgreSQL with pgvector.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore opens a connection pool, registers the pgvector type,
// runs schema migrations, and returns a ready PostgresStore.
// dim is the embedding dimension — must match the model you use
// (e.g. 1024 for nomic-embed-text, 768 for mxbai-embed-large).
func NewPostgresStore(ctx context.Context, databaseURL string, dim int) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	cfg.MaxConns = 5
	cfg.MinConns = 1
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvector.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &PostgresStore{pool: pool}
	if err := s.RunMigrations(ctx, dim); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return s, nil
}

// RunMigrations creates all tables if they don't exist.
// Safe to call on every startup (all statements are idempotent).
func (s *PostgresStore) RunMigrations(ctx context.Context, dim int) error {
	_, err := s.pool.Exec(ctx, schemaSQL(dim))
	return err
}

// BuildVectorIndex creates the IVFFlat ANN index on the embedding column.
// Defer this until the table has at least 300 rows. Safe to call multiple times.
func (s *PostgresStore) BuildVectorIndex(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, indexSQL)
	return err
}

// Close releases the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// SaveMemory inserts a memory row and returns its generated ID.
// If CreatedAt is zero, it defaults to time.Now().
func (s *PostgresStore) SaveMemory(ctx context.Context, mem Memory) (string, error) {
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now()
	}
	id := newUUID()
	tagsJSON, err := marshalEmotionalTags(mem.EmotionalTags)
	if err != nil {
		return "", fmt.Errorf("marshal emotional tags: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO memories
			(id, agent_slug, user_id, conversation_id, memory_type,
			 content, emotional_tags, embedding, importance, created_at, consolidated, durable)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		id, mem.AgentSlug, mem.UserID, mem.ConversationID, mem.MemoryType,
		mem.Content, tagsJSON, pgvector.NewVector(float32Slice(mem.Embedding)),
		mem.Importance, mem.CreatedAt, mem.Consolidated, mem.Durable,
	)
	if err != nil {
		return "", fmt.Errorf("insert memory: %w", err)
	}
	return id, nil
}

// SearchMemoriesByVector returns the nearest memories by cosine distance.
func (s *PostgresStore) SearchMemoriesByVector(ctx context.Context, agentSlug, userID string, embedding []float64, limit int) ([]MemoryWithDistance, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, agent_slug, user_id, conversation_id, memory_type,
		       content, emotional_tags, importance, created_at, consolidated, durable,
		       embedding <=> $1 AS distance
		FROM   memories
		WHERE  agent_slug = $2 AND user_id = $3
		ORDER  BY embedding <=> $1
		LIMIT  $4
	`, pgvector.NewVector(float32Slice(embedding)), agentSlug, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var results []MemoryWithDistance
	for rows.Next() {
		var m Memory
		var tagsJSON []byte
		var distance float64
		if err := rows.Scan(
			&m.ID, &m.AgentSlug, &m.UserID, &m.ConversationID, &m.MemoryType,
			&m.Content, &tagsJSON, &m.Importance, &m.CreatedAt, &m.Consolidated, &m.Durable,
			&distance,
		); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if err := unmarshalEmotionalTags(tagsJSON, &m.EmotionalTags); err != nil {
			return nil, err
		}
		results = append(results, MemoryWithDistance{Memory: m, Distance: distance})
	}
	return results, rows.Err()
}

// CreateExtractionJob inserts a new pending extraction job.
func (s *PostgresStore) CreateExtractionJob(ctx context.Context, conversationID, agentSlug, userID string) (*ExtractionJob, error) {
	job := &ExtractionJob{
		ID:             newUUID(),
		ConversationID: conversationID,
		AgentSlug:      agentSlug,
		UserID:         userID,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO extraction_jobs (id, conversation_id, agent_slug, user_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, job.ID, job.ConversationID, job.AgentSlug, job.UserID, job.Status, job.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert extraction job: %w", err)
	}
	return job, nil
}

// UpdateJobStatus sets status and optional error. Sets completed_at for terminal states.
func (s *PostgresStore) UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now()
		completedAt = &now
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE extraction_jobs SET status = $2, error = $3, completed_at = $4 WHERE id = $1
	`, jobID, status, errorMsg, completedAt)
	return err
}

// GetUnconsolidatedMemories returns all non-consolidated memories for an agent+user.
func (s *PostgresStore) GetUnconsolidatedMemories(ctx context.Context, agentSlug, userID string) ([]Memory, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, agent_slug, user_id, conversation_id, memory_type,
		       content, emotional_tags, importance, created_at, consolidated, durable
		FROM   memories
		WHERE  agent_slug = $1 AND user_id = $2 AND consolidated = FALSE
		ORDER  BY created_at ASC
	`, agentSlug, userID)
	if err != nil {
		return nil, fmt.Errorf("query unconsolidated memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// MarkMemoriesConsolidated bulk-updates a set of memory IDs to consolidated=true.
func (s *PostgresStore) MarkMemoriesConsolidated(ctx context.Context, memoryIDs []string) error {
	_, err := s.pool.Exec(ctx, `UPDATE memories SET consolidated = TRUE WHERE id = ANY($1)`, memoryIDs)
	return err
}

// GetOrCreateRelationshipMetrics returns existing metrics or inserts defaults (0.5/0.5/0.5).
func (s *PostgresStore) GetOrCreateRelationshipMetrics(ctx context.Context, agentSlug, userID string) (*RelationshipMetrics, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO relationship_metrics (agent_slug, user_id, ability, benevolence, integrity, total_conversations, updated_at)
		VALUES ($1, $2, 0.5, 0.5, 0.5, 0, $3)
		ON CONFLICT (agent_slug, user_id) DO NOTHING
	`, agentSlug, userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("upsert relationship metrics: %w", err)
	}
	return s.scanRelationshipMetrics(ctx, agentSlug, userID)
}

// UpdateRelationshipMetrics applies signed deltas to the trust dimensions, clamped to [0,1].
// Increments total_conversations by 1 and sets last_interaction to now.
func (s *PostgresStore) UpdateRelationshipMetrics(ctx context.Context, agentSlug, userID string, shifts map[string]float64) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE relationship_metrics SET
			ability             = GREATEST(0, LEAST(1, ability     + $3)),
			benevolence         = GREATEST(0, LEAST(1, benevolence + $4)),
			integrity           = GREATEST(0, LEAST(1, integrity   + $5)),
			total_conversations = total_conversations + 1,
			last_interaction    = $6,
			updated_at          = $6
		WHERE agent_slug = $1 AND user_id = $2
	`, agentSlug, userID, shifts["ability"], shifts["benevolence"], shifts["integrity"], now)
	return err
}

// SavePersonalityContext upserts the personality summary for an agent+user.
func (s *PostgresStore) SavePersonalityContext(ctx context.Context, agentSlug, userID, personalityContext string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO personality_contexts (agent_slug, user_id, context, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (agent_slug, user_id) DO UPDATE SET context = EXCLUDED.context, updated_at = EXCLUDED.updated_at
	`, agentSlug, userID, personalityContext, time.Now())
	return err
}

// GetPersonalityContext returns the personality summary, or "" if none exists yet.
func (s *PostgresStore) GetPersonalityContext(ctx context.Context, agentSlug, userID string) (string, error) {
	var personalityContext string
	err := s.pool.QueryRow(ctx, `
		SELECT context FROM personality_contexts WHERE agent_slug = $1 AND user_id = $2
	`, agentSlug, userID).Scan(&personalityContext)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return personalityContext, err
}

// GetActiveAgents returns distinct agent slugs that have any stored memories.
func (s *PostgresStore) GetActiveAgents(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT agent_slug FROM memories`)
	if err != nil {
		return nil, fmt.Errorf("query active agents: %w", err)
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

// ─── helpers ─────────────────────────────────────────────────────────────────

func (s *PostgresStore) scanRelationshipMetrics(ctx context.Context, agentSlug, userID string) (*RelationshipMetrics, error) {
	var m RelationshipMetrics
	err := s.pool.QueryRow(ctx, `
		SELECT agent_slug, user_id, ability, benevolence, integrity,
		       total_conversations, last_interaction, updated_at
		FROM   relationship_metrics WHERE agent_slug = $1 AND user_id = $2
	`, agentSlug, userID).Scan(
		&m.AgentSlug, &m.UserID, &m.Ability, &m.Benevolence, &m.Integrity,
		&m.TotalConversations, &m.LastInteraction, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan relationship metrics: %w", err)
	}
	return &m, nil
}

func scanMemories(rows pgx.Rows) ([]Memory, error) {
	var memories []Memory
	for rows.Next() {
		var m Memory
		var tagsJSON []byte
		if err := rows.Scan(
			&m.ID, &m.AgentSlug, &m.UserID, &m.ConversationID, &m.MemoryType,
			&m.Content, &tagsJSON, &m.Importance, &m.CreatedAt, &m.Consolidated, &m.Durable,
		); err != nil {
			return nil, fmt.Errorf("scan memory row: %w", err)
		}
		if err := unmarshalEmotionalTags(tagsJSON, &m.EmotionalTags); err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

func marshalEmotionalTags(tags *EmotionalTags) ([]byte, error) {
	if tags == nil {
		return nil, nil
	}
	return json.Marshal(tags)
}

func unmarshalEmotionalTags(data []byte, dst **EmotionalTags) error {
	if len(data) == 0 {
		return nil
	}
	var tags EmotionalTags
	if err := json.Unmarshal(data, &tags); err != nil {
		return fmt.Errorf("unmarshal emotional tags: %w", err)
	}
	*dst = &tags
	return nil
}

func float32Slice(f64 []float64) []float32 {
	out := make([]float32, len(f64))
	for i, v := range f64 {
		out[i] = float32(v)
	}
	return out
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
