// Example composition root for an axon-memo service.
//
// Demonstrates how to wire up the memory service with:
//   - A MemoryStore implementation (in-memory for this example)
//   - A ConversationSource (stub — replace with HTTP client to axon-chat)
//   - TextGenerator and EmbeddingGenerator functions (stubs — replace with LLM client)
//
// For a production deployment, replace the stubs with real implementations:
//   - InMemoryStore → PostgreSQL with pgvector
//   - stubConversationSource → memo.NewConversationClient(chatURL)
//   - stubTextGenerator → Ollama, Claude, or other LLM provider
//   - stubEmbeddingGenerator → Ollama nomic-embed-text or similar
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	memo "github.com/benaskins/axon-memo"
)

func main() {
	// 1. Storage backend
	store := NewInMemoryStore()

	// 2. Conversation source — reads messages from axon-chat.
	// In production: memo.NewConversationClient("http://chat.studio.internal:8080")
	source := &stubConversationSource{}

	// 3. LLM functions — wired to your provider of choice.
	// In production: wire to Ollama, Claude API, or another LLM.
	generate := stubTextGenerator
	embed := stubEmbeddingGenerator

	// 4. Assemble service components
	extractor := memo.NewExtractor(store, source, generate, embed)
	retriever := memo.NewRetriever(store, embed)
	consolidator := memo.NewConsolidator(store, source, generate, embed)

	// 5. Create server and wire optional dependencies
	server := memo.NewServer(store, extractor, retriever, consolidator)
	// server.Analytics = memo.NewAnalyticsClient("http://look.studio.internal:8084")

	// 6. Start consolidation scheduler (runs daily at 2AM)
	server.StartScheduler()
	defer server.StopScheduler()

	// 7. Serve
	handler := server.Handler()

	port := envOr("PORT", "8086")
	slog.Info("starting memo service", "port", port)

	srv := &http.Server{Addr: ":" + port, Handler: handler}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		slog.Info("shutting down")
		srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
