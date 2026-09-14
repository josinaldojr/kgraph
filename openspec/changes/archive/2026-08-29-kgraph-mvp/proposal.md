## Why

AI-assisted code review and analysis today requires loading large swaths of a repository into an LLM's context window, which is expensive, slow, and drowns the signal (relevant relationships, side effects, schema touches) in irrelevant surrounding code. A compact, queryable knowledge graph of code entities and relationships — with short LLM-generated summaries instead of raw source — lets an AI reviewer get a targeted, token-budgeted view of "what does this component do and what does it touch" without reading the whole repo.

## What Changes

- New standalone Go CLI, `kgraph`, as its own Go module — kept separate from `knowledge-cli`/`kv` (which models OpenSpec *decision* memory) so code-structure memory and decision memory stay independent domains, consistent with `kv`'s own "don't silently bridge feature sets" principle.
- Go-only source parsing for v1, using the standard library (`go/ast`, `go/parser`, `go/token`) plus `golang.org/x/tools/go/packages` for call-edge resolution — no `tree-sitter`/cgo dependency. Scope is explicitly single-language (Go repositories) for this MVP; other languages are out of scope.
- Typed in-memory graph (`internal/graph`) with nodes (`Package`, `Struct`, `Interface`, `Function`, `Field`, `Table`, `Column`, `Endpoint`, `ExternalDependency`) and typed edges (`imports`, `calls`, `embeds`, `implements`, `has_method`, `has_field`, `maps_to_table`, `references_fk`, `reads_table`, `writes_table`, `exposes_endpoint`), each node carrying file path, line range, signature, and a content hash.
- SQLite persistence (`internal/store`, `modernc.org/sqlite`, cgo-free) with `nodes`/`edges` tables storing `type` + JSON `properties`, keyed so re-running `build` on an unchanged file is a no-op.
- ORM/migration extraction: detect `gorm`/`db` struct tags and map them to `Table`/`Column`/`references_fk` edges; parse `.sql` migration files to reconcile schema-derived nodes with struct-tag-derived ones.
- Provider-driven summarization: `kgraph summarize pending` exports nodes/files/modules needing a 1–3 line summary as JSON; `kgraph summarize apply` reads back provider-written summaries and stores them, cached by content hash so unchanged code is never re-summarized. No LLM API key lives inside `kgraph` — the AI provider already driving the session (Claude Code or similar) does the actual summarizing.
- Lexical `SearchNodes(query, topK)`: term-overlap scoring against node summaries/names — no embeddings, no external embedding API.
- `GetContext(target, hops, maxTokens)`: resolves a target (file, struct/function name, or semantic search), expands the subgraph N hops out, and renders a token-budgeted context string from summaries + direct relations (callers, callees, tables touched) — never raw source.
- `Update(repoPath, dbPath)`: identifies changed files (via `os/exec` `git diff --name-only`, fetched internally rather than passed in), reprocesses only their nodes, and invalidates summaries for the changed nodes and their direct neighbors.
- CLI (`github.com/spf13/cobra`): `kgraph build <repo_path>`, `kgraph update`, `kgraph context <target> [--hops N] [--max-tokens N]`, `kgraph search <query>`.

## Capabilities

### New Capabilities
- `code-graph-extraction`: parsing Go source into typed nodes/edges (packages, structs, interfaces, functions, fields, call edges) plus ORM-tag and SQL-migration-derived schema nodes.
- `graph-storage`: in-memory graph representation and its SQLite persistence (schema, hashing/change-detection, idempotent rebuilds).
- `entity-summarization`: provider-driven (export/apply, not a direct LLM API call) summary generation per node/file/module, cached and invalidated by content hash.
- `node-search`: lexical `SearchNodes` scoring by term overlap against node summaries/names.
- `context-assembly`: `GetContext` target resolution, N-hop subgraph expansion, and token-budgeted rendering.
- `incremental-update`: `Update` change detection and scoped reprocessing/invalidation.
- `kgraph-cli`: the `build`/`update`/`context`/`search` command surface and their flags/output contracts.

### Modified Capabilities
(none — this is a new standalone tool; no existing specs change)

## Impact

- New standalone repository/module (not a change to `knowledge-cli` or any existing project); no existing project's code is modified.
- No LLM/embedding API key dependency: summarization is provider-driven (export/apply round trip), and search is lexical. `build`/`update`/`context`/`search` all run with zero external API calls; only `summarize apply` requires an AI provider to have generated the summary text being applied.
- New local artifact: a per-analyzed-repo SQLite database file (path TBD in design) holding the graph, summaries, and embeddings — this file is derived/regenerable, not meant to be a source of truth checked into the analyzed repo.
- Scope is single-language (Go) for this MVP; type resolution is best-effort via `go/packages`, not full `go/types` correctness; no web UI.
