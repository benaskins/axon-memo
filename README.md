# axon-memo

Long-term memory extraction and consolidation for LLM-powered agents.

Extracts facts from conversations, deduplicates them, and serves consolidated memories on demand.

## Install

```
go get github.com/benaskins/axon-memo@latest
```

Requires Go 1.24+.

## Usage

```go
extractor := mem.NewExtractor(ollamaClient, extractionModel)
retriever := mem.NewRetriever(memoryStore, ollamaClient, embeddingModel)
consolidator := mem.NewConsolidator(memoryStore, ollamaClient, consolidationModel)

srv := mem.NewServer(memoryStore, extractor, retriever, consolidator)
http.Handle("/", srv)
```

### Key types

- `Memory`, `RecalledMemory` — memory domain types
- `MemoryStore` — persistence interface (embedding-aware)
- `Extractor` — extracts memories from conversations via LLM
- `Retriever` — semantic recall with embedding similarity
- `Consolidator` — deduplicates and merges related memories
- `Server` — HTTP server with extract, recall, and consolidation endpoints

## License

Apache 2.0 — see [LICENSE](LICENSE).
