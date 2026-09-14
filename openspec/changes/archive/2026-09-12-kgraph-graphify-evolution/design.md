## Context

kgraph is a Go-based code knowledge graph that parses repositories (Go, Java, TypeScript, Python), extracts a typed graph of code entities and relationships, persists it to SQLite, and serves it via HTTP. It has a solid foundation: multi-language parser factory, BFS-based context generation with token budgets, incremental updates, and a live visualization server.

The inspiration is [Graphify](https://github.com/Graphify-Labs/graphify), which adds intelligence layers over a code graph: rationale extraction, edge confidence, community detection, god nodes, query/path/explain commands, prompt export, and MCP server integration. This change brings those concepts to kgraph while preserving its existing architecture, Go-native approach, and zero-LLM-for-code principle.

Current state:
- `internal/graph/`: Node, Edge, Graph (in-memory), IDs, hashes
- `internal/parser/`: Factory pattern with Go (go/ast), Python (regex), TypeScript (regex), Java (regex)
- `internal/context/`: BFS expansion + token-budgeted rendering
- `internal/store/`: SQLite persistence (nodes, edges, summaries, build_meta)
- `internal/server/`: HTTP API, SSE broadcaster, hub mode, poller
- `internal/build/`: Build + incremental update pipeline
- `internal/summarizer/`: External summarization pipeline (export pending, apply results)
- `cmd/kgraph/`: CLI with build, update, context, search, summarize, serve, projects, prune

## Goals / Non-Goals

**Goals:**
- Add rationale extraction (NOTE/WHY/HACK comments as first-class nodes)
- Add edge confidence tagging (EXTRACTED vs INFERRED)
- Add graph analytics (degree, god nodes, community detection)
- Add query commands (query, path, explain, prompt)
- Add graph export (graph.json, GRAPH_REPORT.md)
- Add MCP server for AI assistant integration
- Preserve all existing functionality (build, update, context, search, serve)
- Keep zero-LLM for code parsing (LLM optional for docs/media only)

**Non-Goals:**
- Tree-sitter integration (future change — keep existing parsers)
- LLM-powered semantic query (future — use heuristic query for now)
- Real-time collaboration or multi-user editing
- Replacing the SQLite store with a graph database
- Supporting 40+ languages (keep current 4, add tree-sitter fallback later)
- Video/image/PDF extraction (Graphify-specific, out of scope)

## Decisions

### Decision 1: Properties Bag for Analytics Metadata

**Choice**: Store community, degree, god_node, and rationale_count as keys in Node.Properties map[string]any.

**Rationale**: The Properties bag already exists, is serialized to JSON natively, and doesn't require schema migration. Analytics metadata is about the node, not part of the code model. Typed getters (Node.Community(), Node.IsGodNode()) provide convenience without schema changes.

**Alternatives considered**:
- Separate `node_analytics` table: More normalized but adds JOIN complexity for every query. Overkill for metadata that's always loaded with the node.
- Typed fields on Node: Would require schema migration and makes the Node struct carry analytics concerns alongside code model concerns.

### Decision 2: Confidence as Edge Field

**Choice**: Add `Confidence string` as a direct field on Edge (not in Properties).

**Rationale**: Confidence is as fundamental to the edge model as Type. It's always present, always needed, and benefits from being a first-class field rather than a bag entry. The Edge struct is small enough that adding one field is clean.

**Alternatives considered**:
- Edge.Properties["confidence"]: Works but loses type safety and makes confidence feel optional.
- Separate edge_confidence table: Over-engineered for a single string field.

**Migration**: `ALTER TABLE edges ADD COLUMN confidence TEXT NOT NULL DEFAULT 'EXTRACTED'`. Existing edges get EXTRACTED by default; next rebuild corrects INFERRED edges.

### Decision 3: Three-Stage Build Pipeline (Parse → Enrich → Analyze)

**Choice**: Build runs as Parse (existing) → Enrich (new) → Analyze (new) → Persist (existing).

**Rationale**: Separation of concerns. Parse extracts code structure. Enrich adds semantic metadata (rationale, confidence). Analyze computes structural properties (degree, communities, god nodes). Each stage reads from the graph and writes to it, with clear boundaries.

**Alternatives considered**:
- Inline enrichment in parser: Mixes concerns, harder to test enrichment independently.
- Post-serve analytics: Would require lazy computation and cache invalidation. Pre-computing at build time is simpler for a CLI tool.

### Decision 4: Heuristic Community Detection (Not Leiden)

**Choice**: Implement community detection using label propagation with resolution-based merging, not the full Leiden algorithm.

**Rationale**: Leiden requires a quality function optimizer and iterative refinement that's complex to implement correctly in Go. A heuristic approach gives good enough results for code graphs (which are sparse and have natural clusters) without the implementation complexity: every node starts in its own community, then repeatedly adopts the community held by the plurality of its neighbors (by edge count, both directions) until the assignment stabilizes; a `resolution`-derived size threshold then folds any community smaller than it into whichever neighboring community it's most strongly connected to, so label propagation's characteristic handful of stray singletons don't each end up as their own community. Can be upgraded to Leiden later.

**Alternatives considered**:
- Full Leiden implementation (~300 LOC, complex, needs testing against reference implementations).
- Calling Python igraph via subprocess: Adds runtime dependency, complicates deployment.
- Connected components only: Too coarse — every repo would be one community or a few.

**Correction (graphify-evolution-followups-2, 2026-09-13)**: This decision originally described "connected-component expansion with modularity-based refinement." What was actually implemented and shipped (`internal/analytics/community.go`'s `DetectCommunities`) is label propagation, not connected-component expansion — the text above has been corrected to match. See that follow-up change for the review that caught the discrepancy.

### Decision 5: Query as Evolution of Context

**Choice**: The `query` command builds on the existing BFS + token-budget engine in `internal/context`, adding lexical relevance scoring and community-aware scoping.

**Rationale**: The existing `context` package already does BFS expansion with hop limits and token budgets. Query adds: (1) relevance scoring so the most relevant nodes appear first, (2) community scoping so related nodes stay together, (3) richer rendering with rationale and confidence.

**Alternatives considered**:
- Separate query engine: Would duplicate BFS, token estimation, and rendering logic.
- LLM-powered query routing: Adds LLM dependency. Future enhancement, not for v1.

### Decision 6: Dijkstra for Path Finding

**Choice**: Implement shortest-path using Dijkstra's algorithm with edge weights from the existing edgeWeight() function.

**Rationale**: The graph is small enough (typically <10k nodes) that Dijkstra is fast. Edge weights already exist (calls=3, has_method=3, structural=2, imports=1) and give meaningful shortest paths that prefer structural relationships over import references.

**Alternatives considered**:
- BFS (unweighted): Ignores edge importance — shortest path through 3 import edges would rank above 2 call edges.
- A* with spatial heuristics: No spatial embedding available. Dijkstra is sufficient.

### Decision 7: Prompt as Formatted Explain

**Choice**: The `prompt` command is a formatting layer over `explain` that structures the output as an LLM-ready prompt with sections (Context, Relations, Rationale, Community).

**Rationale**: Explain already gathers all the information. Prompt just formats it for clipboard consumption. Keeps the logic DRY.

### Decision 8: MCP Server as Thin Wrapper

**Choice**: MCP server in `internal/mcp/` wraps existing context/analytics/export functions. No new graph logic lives in the MCP layer.

**Rationale**: All the intelligence is in `internal/context/`, `internal/analytics/`, `internal/export/`. The MCP layer just maps tool calls to function calls and serializes results. Keeps the MCP server replaceable (swap stdio for HTTP, swap SDK, etc.).

### Decision 9: Export Serializes the Enriched Graph

**Choice**: `graph.json` export serializes the full graph (nodes with all Properties, edges with Confidence, summaries, community metadata) as a single JSON file.

**Rationale**: The graph is the single source of truth. Export is just serialization. The JSON file is portable and can be consumed by any tool (AI assistants, visualization, analysis scripts).

### Decision 10: Report as Generated Markdown

**Choice**: `GRAPH_REPORT.md` is generated from the graph analytics (god nodes, communities, surprising cross-community connections, suggested questions).

**Rationale**: The report is a human-readable summary of what the graph analysis found. It's generated, not hand-written. Suggested questions come from the graph structure (e.g., "What connects community A to community B?").

## Risks / Trade-offs

**[Risk] Community detection quality**
→ Heuristic approach may produce less balanced communities than Leiden. Mitigation: Start with heuristic, benchmark against test repos, upgrade to Leiden if quality is insufficient. The `kgraph analyze --recluster` command allows re-running with different parameters.

**[Risk] Rationale extraction misses non-standard comment formats**
→ Regex-based extraction only catches `NOTE:`, `WHY:`, `HACK:` prefixes. Mitigation: Document the supported formats. Future enhancement: LLM-based rationale extraction for docs.

**[Risk] Performance of analytics on large graphs**
→ Community detection is O(N+E). For very large repos (>50k nodes), this could add significant time to the build. Mitigation: Analytics is a separate stage that can be skipped (`kgraph build --no-analyze`). For most repos (<10k nodes), the overhead is <2 seconds.

**[Risk] Schema migration for existing users**
→ Adding `confidence` column to edges table. Mitigation: ALTER TABLE with DEFAULT 'EXTRACTED' is safe and backward-compatible. Existing databases work without rebuild; next rebuild populates correct confidence values.

**[Risk] MCP SDK dependency**
→ Adding `github.com/mark3labs/mcp-go` as a dependency. Mitigation: The MCP package is isolated in `internal/mcp/` and doesn't affect core build/query logic. Can be replaced if the SDK becomes unmaintained.

**[Trade-off] Pre-computed analytics vs. on-demand**
→ Pre-computing at build time means analytics are always available but add build time. On-demand would be faster builds but slower queries. Chose pre-computation because builds are infrequent and queries should be instant.

**[Trade-off] Properties bag vs. typed fields**
→ Properties bag is flexible but lacks compile-time safety. Typed fields are safe but require migrations. Chose Properties for analytics metadata (flexible, schema-free) and typed fields for Confidence (fundamental, always needed).
