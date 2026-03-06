# axon-memo

Long-term memory extraction and consolidation for LLM-powered agents. Part of [lamina](https://github.com/benaskins/lamina) — each axon package can be used independently.

Extracts facts from conversations, deduplicates them, and serves consolidated memories on demand.

## Install

```
go get github.com/benaskins/axon-memo@latest
```

Requires Go 1.24+.

## Usage

```go
extractor := memo.NewExtractor(ollamaClient, extractionModel)
retriever := memo.NewRetriever(memoryStore, ollamaClient, embeddingModel)
consolidator := memo.NewConsolidator(memoryStore, ollamaClient, consolidationModel)

srv := memo.NewServer(memoryStore, extractor, retriever, consolidator)
http.Handle("/", srv)
```

### Key types

- `Memory`, `RecalledMemory` — memory domain types
- `MemoryStore` — persistence interface (embedding-aware)
- `Extractor` — extracts memories from conversations via LLM
- `Retriever` — semantic recall with embedding similarity
- `Consolidator` — deduplicates and merges related memories
- `Server` — HTTP server with extract, recall, and consolidation endpoints

## Acknowledgements

Memory architecture inspired by the [A-MEM](https://arxiv.org/abs/2502.12110) paper on agentic memory for LLM agents (Xu et al., 2025).

## License

MIT — see [LICENSE](LICENSE).
