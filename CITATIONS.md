# Citations

> **Source of truth:** [lamina/docs/CITATIONS.md](https://github.com/benaskins/lamina/blob/main/docs/CITATIONS.md)

Research referenced in axon-memo design and implementation.

## Trust model

**Mayer, R. C., Davis, J. H., & Schoorman, F. D. (1995).** An integrative model of organizational trust. *Academy of Management Review*, 20(3), 709-734.

Used for: `RelationshipMetrics` dimensions (ability, benevolence, integrity). These three factors are the antecedents of trust in the Mayer model. Applied here as observable behavioral dimensions for agent-user interactions — not as a claim that agents experience trust.

Referenced in: `types.go`, `extractor.go`, `consolidator.go`, `retrieval.go`

## Memory architecture

**Xu, Z., et al. (2025).** A-MEM: Agentic Memory for LLM Agents. *arXiv:2502.12110*.

Used for: Overall memory architecture pattern (extract from conversations, consolidate over time, recall on demand). The three-phase pipeline and memory type categorization (episodic, semantic, emotional) draw from this work.

Referenced in: `README.md`, `extractor.go`, `consolidator.go`, `retrieval.go`
