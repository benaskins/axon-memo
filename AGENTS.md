# axon-memo

Long-term memory extraction and consolidation for LLM agents.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files

- `consolidator.go` — memory consolidation logic
- `analytics.go` — analytics event tracking
- `conversation_client.go` — client for fetching conversation history
