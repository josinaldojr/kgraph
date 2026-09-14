## Why

kgraph is a solid code knowledge graph for Go (and multi-language) repositories, but it lacks the intelligence layer that makes a knowledge graph truly useful for navigating codebases. Inspired by [Graphify](https://github.com/Graphify-Labs/graphify), this change transforms kgraph from a "parse and store" tool into a "parse, enrich, analyze, query, and export" platform — making the knowledge path through codebases dramatically shorter for AI assistants and developers.

The key gap: kgraph knows *what* is in the code (nodes, edges, types) but not *why* (rationale, design decisions), *how important* (god nodes, centrality), or *how things relate* (communities, subsystems). This change closes that gap.

## What Changes

- **Rationale Extraction**: Comments tagged `NOTE:`, `WHY:`, `HACK:`, docstrings, and ADR/RFC references become first-class `Rationale` nodes linked to the code they explain, with an `explains` edge type.
- **Edge Confidence**: Every edge gains a `Confidence` field (`EXTRACTED` from AST, `INFERRED` from cross-file resolution), making the graph's provenance transparent.
- **Graph Analytics**: Degree calculation, god-node detection (top-N most connected nodes), and community detection (heuristic clustering) run as a post-build analysis pass. Results are stored as node properties.
- **Query Commands**: New CLI commands `query`, `path`, `explain`, and `prompt` let users ask natural-language questions, trace shortest paths between nodes, get rich node context, and copy context as LLM-ready prompts.
- **Export**: `graph.json` (portable full graph) and `GRAPH_REPORT.md` (summary with god nodes, communities, surprising connections) exports.
- **MCP Server**: Expose the graph as MCP tools (`query_graph`, `get_node`, `get_neighbors`, `shortest_path`, `explain_node`, `export_graph`) for AI coding assistants.
- **Server API Extensions**: New HTTP endpoints `/api/query`, `/api/path`, `/api/explain` on the existing serve infrastructure.

## Capabilities

### New Capabilities

- `edge-confidence`: Edge provenance tagging — marking each edge as EXTRACTED (from AST) or INFERRED (from cross-file analysis).
- `rationale-extraction`: Extracting NOTE/WHY/HACK comments, docstrings, and ADR references as first-class Rationale nodes with `explains` edges.
- `graph-analytics`: Post-build structural analysis — degree calculation, god-node detection, and community detection via heuristic clustering.
- `query-engine`: CLI and API query commands — `query` (natural-language subgraph), `path` (shortest path), `explain` (rich node context), `prompt` (LLM-ready clipboard output).
- `graph-export`: Portable graph serialization — `graph.json` (full graph with all metadata) and `GRAPH_REPORT.md` (summary report).
- `mcp-server`: MCP tool server exposing graph operations to AI coding assistants via stdio or HTTP transport.

### Modified Capabilities

- `graph-model`: Node Properties gain community, degree, god_node, rationale_count; Edge gains Confidence field.
- `build-pipeline`: Build now includes enrich and analyze stages after parse.
- `server-api`: HTTP API gains /api/query, /api/path, /api/explain endpoints.

## Impact

- **Code**: New packages `internal/enrich/`, `internal/analytics/`, `internal/export/`, `internal/mcp/`. Extended packages: `internal/graph/`, `internal/context/`, `internal/store/`, `internal/server/`, `internal/build/`, `cmd/kgraph/`.
- **Schema**: SQLite edges table gains `confidence` column. Nodes table unchanged (analytics metadata stored in Properties JSON).
- **Dependencies**: No new external dependencies for core features. MCP server requires a Go MCP SDK (e.g., `github.com/mark3labs/mcp-go`). Community detection uses a custom heuristic implementation (no external graph library).
- **APIs**: New CLI commands: `query`, `path`, `explain`, `prompt`, `report`, `export`. New HTTP endpoints. New MCP tools.
- **Breaking**: None. All changes are additive. Existing `context`, `search`, `serve` commands continue to work unchanged.
