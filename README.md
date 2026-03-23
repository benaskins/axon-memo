# axon-memo

> Domain package · Part of the [lamina](https://github.com/benaskins/lamina-mono) workspace

Long-term memory extraction and consolidation for LLM agents. Extracts episodic, semantic, and emotional memories from conversations, deduplicates them through consolidation, and serves ranked memories on demand via semantic recall. Relationship trustworthiness tracking uses the Mayer, Davis & Schoorman (1995) ABI model.

## Getting started

```
go get github.com/benaskins/axon-memo@latest
```

Requires Go 1.26.1+.

axon-memo is a domain package — it provides types, interfaces, and HTTP handlers but no `main`. You assemble it in your own composition root by wiring a `MemoryStore`, `TextGenerator`, and `EmbeddingGenerator`. See [`example/`](example/) for a working setup.

```go
// Wire LLM functions and a MemoryStore at the composition root.
var store memo.MemoryStore       // e.g. PostgreSQL + pgvector
var source memo.ConversationSource // reads messages from axon-chat
var generate memo.TextGenerator    // LLM text completion
var embed memo.EmbeddingGenerator  // LLM embedding model

extractor := memo.NewExtractor(store, source, generate, embed)
retriever := memo.NewRetriever(store, embed)
consolidator := memo.NewConsolidator(store, source, generate, embed)

server := memo.NewServer(store, extractor, retriever, consolidator, memo.WithAnalytics(analytics))
server.StartScheduler() // nightly consolidation at 2AM
defer server.StopScheduler()

mux := http.NewServeMux()
mux.Handle("/", server.Handler())
log.Fatal(http.ListenAndServe(":8086", mux))
```

## CLI

The `memo` CLI at [`cmd/memo/`](cmd/memo/) provides `memo store` and `memo recall` commands that talk to a running axon-memo service over HTTP. Install with:

```
go install github.com/benaskins/axon-memo/cmd/memo@latest
```

## Key types

- `Memory`, `RecalledMemory` — memory domain types (episodic, semantic, emotional)
- `MemoryStore` — persistence interface (vector search, relationship metrics, consolidation)
- `Extractor` — extracts memories from conversations via LLM
- `Retriever` — semantic recall with embedding similarity and relevance ranking
- `Consolidator` — deduplicates and merges related memories
- `Scheduler` — periodic consolidation across active agents and users
- `Server` — HTTP handlers for extract, recall, store, and consolidation endpoints
- `ConversationClient` — HTTP client for fetching conversation data
- `AnalyticsClient` — HTTP client for emitting analytics events
- `TextGenerator`, `EmbeddingGenerator` — function types wired to an LLM at the composition root
- `ExtractRequest`, `StoreRequest`, `ConsolidateRequest` — HTTP request types
- `RelationshipMetrics` — trustworthiness tracking (Mayer, Davis & Schoorman model)

## License

MIT — see [LICENSE](LICENSE).