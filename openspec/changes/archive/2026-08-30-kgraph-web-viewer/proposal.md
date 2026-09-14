## Why

`kgraph` builds a rich code knowledge graph (nodes, edges, per-node notes) but the only way to see any of it today is one-shot CLI output (`context`, `search`) — there is no way to browse the whole graph, see how it grows as `build`/`update` run, or spot which nodes still lack a note. A live, browsable visualization turns the graph from a context source an AI reads into something a person can actually explore.

## What Changes

- Add `kgraph serve`: a long-running, read-only HTTP server (embedded frontend via `go:embed`, single binary, no npm build step) that visualizes the graph as a drill-down hierarchy (Package → File → Struct/Interface/Function, with Table/Endpoint/ExternalDependency as separate lenses), backed by the existing SQLite store opened read-only alongside a concurrently running `build`/`update` process (WAL mode already supports this: one writer, many readers).
- The server polls `build_meta.last_build_at` (already bumped by both `build` and `update`) to detect new data and pushes a refresh to connected browsers over Server-Sent Events — no new schema needed for change detection.
- The UI reuses existing capabilities rather than reimplementing them: a search box backed by `SearchNodes`, and a node detail panel showing the node's note/summary, signature, file/line, and in/out edges (the same information `context` already assembles, rendered for a person instead of an AI).
- Extend `summarize pending`/`summarize apply` coverage so every node type that can meaningfully carry its own note gets one. `Package` nodes are already covered today (aggregated from file summaries at the existing `module` level, keyed by the Package node's own ID) — the actual gap is `Table`, `Endpoint`, and `ExternalDependency`, which currently have no summary path at all. Each gets its own definition of "source text": a `Table`'s is its column list plus foreign-key edges; an `Endpoint`'s is its method, path, and handler signature; an `ExternalDependency`'s is its module path and declared version.
- `Field` and `Column` nodes remain visible in the graph and in their parent's detail view, but intentionally do **not** get their own note — they're covered by their containing `Struct`/`Table`'s note. This avoids one-line-per-field noise ("field Name is a string") while still satisfying "every meaningful node has a note."

## Capabilities

### New Capabilities
- `graph-visualization`: the live, browsable web view of the knowledge graph — drill-down navigation, search-to-focus, node detail (note + relations), and live refresh as `build`/`update` add or change nodes.

### Modified Capabilities
- `kgraph-cli`: adds the `kgraph serve` command.
- `entity-summarization`: broadens the pending-summary export/apply pipeline from `Struct`/`Interface`/`Function` to also include `Table`, `Endpoint`, and `ExternalDependency`, each with a type-appropriate source-text definition; `Field`/`Column` remain explicitly excluded from having their own summary. (`Package` already has a note today via the existing file→module aggregation — no change needed there.)

## Impact

- New Go packages inside the `kgraph` module: an HTTP server/router, a small embedded static frontend (HTML/CSS/JS, no framework), and SSE plumbing for live refresh.
- `internal/store`: needs a read-only `Open` path (or equivalent) for `serve` so it never risks a write against a database a `build`/`update` process might be writing to concurrently.
- `internal/summarizer` and the `summarize pending`/`apply` commands: extended to enumerate and render source text for the three newly-covered node types (`Table`, `Endpoint`, `ExternalDependency`).
- No changes to `internal/graph`, `internal/parser`, `internal/context`, or `node-search` — `serve` is additive and reuses them as-is.
