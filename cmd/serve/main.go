// Command serve runs the axon-memo memory service.
//
// Required environment variables:
//
//	DATABASE_URL      PostgreSQL connection string (with pgvector extension)
//	ANTHROPIC_API_KEY Anthropic API key (for extraction/consolidation)
//	CONVERSATION_URL  Base URL of the conversation service
//	OLLAMA_URL        Ollama base URL for embeddings (e.g. http://localhost:11434)
//
// Optional:
//
//	PORT              HTTP listen port (default: 8086)
//	EMBED_MODEL       Ollama embedding model (default: nomic-embed-text)
//	EMBEDDING_DIM     Embedding vector dimension (default: 768 for nomic-embed-text)
//	TEXT_MODEL        Anthropic model (default: claude-haiku-4-5-20251001)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	memo "github.com/benaskins/axon-memo"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	databaseURL := requireEnv("DATABASE_URL")
	apiKey := requireEnv("ANTHROPIC_API_KEY")
	conversationURL := requireEnv("CONVERSATION_URL")
	ollamaURL := requireEnv("OLLAMA_URL")

	port := envOr("PORT", "8086")
	dim := envInt("EMBEDDING_DIM", 768)
	embedModel := envOr("EMBED_MODEL", "nomic-embed-text")
	textModel := anthropic.Model(envOr("TEXT_MODEL", string(anthropic.ModelClaudeHaiku4_5_20251001)))

	slog.Info("starting axon-memo",
		"port", port,
		"embedding_dim", dim,
		"embed_model", embedModel,
		"text_model", textModel,
	)

	store, err := memo.NewPostgresStore(ctx, databaseURL, dim)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	generate := func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     textModel,
			MaxTokens: int64(maxTokens),
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
			},
			Temperature: anthropic.Float(temperature),
		})
		if err != nil {
			return "", fmt.Errorf("anthropic message: %w", err)
		}
		if len(msg.Content) == 0 {
			return "", fmt.Errorf("empty response from model")
		}
		return msg.Content[0].Text, nil
	}

	embed := ollamaEmbed(ollamaURL, embedModel)

	source := memo.NewConversationClient(conversationURL)
	extractor := memo.NewExtractor(store, source, generate, embed)
	retriever := memo.NewRetriever(store, embed)
	consolidator := memo.NewConsolidator(store, source, generate, embed)
	srv := memo.NewServer(store, extractor, retriever, consolidator)

	srv.StartScheduler()
	defer srv.StopScheduler()

	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      srv.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutCancel()
		return httpSrv.Shutdown(shutCtx)
	}
}

// ollamaEmbed returns an EmbeddingGenerator backed by the Ollama /api/embeddings endpoint.
func ollamaEmbed(baseURL, model string) memo.EmbeddingGenerator {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return func(ctx context.Context, text string) ([]float64, error) {
		body, _ := json.Marshal(map[string]string{"model": model, "prompt": text})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ollama embed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("ollama embed status %d: %s", resp.StatusCode, b)
		}

		var result struct {
			Embedding []float64 `json:"embedding"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode embedding: %w", err)
		}
		return result.Embedding, nil
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("missing required environment variable", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
