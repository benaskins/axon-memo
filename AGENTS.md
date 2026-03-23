# axon-memo

Long-term memory extraction and consolidation for LLM agents.

## Build & Test

```bash
go test ./...
go vet ./...
go mod tidy
```

## Dependencies

Main dependencies from go.mod:
- `github.com/benaskins/axon` - Core web framework and utilities
- `github.com/benaskins/axon-fact` - Event sourcing support
- `github.com/spf13/cobra` - CLI framework (for cmd/memo)

## Key Files

Core domain logic:
- `types.go` — core domain types and data structures
- `store.go` — MemoryStore interface definition
- `extractor.go` — memory extraction from conversations
- `retrieval.go` — semantic memory recall and ranking
- `consolidator.go` — memory consolidation and deduplication
- `scheduler.go` — periodic consolidation scheduling
- `server.go` — HTTP handlers and routing

LLM integration:
- `llm.go` — LLM function types and prompt building
- `conversation_client.go` — client for fetching conversation history

Analytics and events:
- `analytics.go` — analytics event tracking
- `domain_events.go` — domain event definitions

CLI:
- `cmd/memo/main.go` — CLI entry point
- `cmd/memo/store.go` — memory storage commands
- `cmd/memo/recall.go` — memory recall commands

Example:
- `example/main.go` — example service composition
- `example/store.go` — example MemoryStore implementation
- `example/stubs.go` — stub implementations for testing