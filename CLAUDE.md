@AGENTS.md

## Conventions
- Extract/consolidate/recall pipeline  - memories flow through all three stages
- Trust model uses Mayer ABI dimensions: ability, benevolence, integrity
- Default agent identifier is `claude-code` when none specified
- CLI at `cmd/memo/` provides `memo store` and `memo recall` commands
- `POST /api/memory/store` is the direct store endpoint (bypasses conversation extraction)
- Durable memories (`Durable: true`) skip recency decay and emotional boost in ranking

## Constraints
- Trust model is grounded in Mayer, Davis & Schoorman (1995)  - do not replace with ad-hoc relationship dimensions
- Depends on axon (HTTP lifecycle) and axon-fact (event sourcing)  - do not add dependencies on other axon-* service packages
- Memory ranking weights are intentional  - do not flatten or simplify the scoring algorithm without justification

## Testing
- `go test ./...` runs all tests
- `go vet ./...` for lint
- Extractor and consolidator tests require LLM function stubs  - see `example/stubs.go`
