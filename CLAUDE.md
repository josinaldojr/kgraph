# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

kgraph parses source repositories, extracts a typed graph of code entities and relationships, and persists it to SQLite. The graph is served as targeted, token-budgeted context for AI code reviewers — the design intent is that reviewers consume the graph, never raw source. See `README.md` for the full CLI/user-facing docs and `openspec/specs/*/spec.md` for the authoritative behavioral specs per capability.

## Commands

```bash
make build          # builds ./bin/kgraph
make install        # go install to $(go env GOBIN)/GOPATH/bin
make test           # go test ./...
make fmt            # go fmt ./...
make vet            # go vet ./...
make tidy           # go mod tidy
make help           # list all targets
```

Run a single package's tests or a single test directly with `go test`, e.g.:

```bash
go test ./internal/parser/go/...
go test ./internal/build/... -run TestUpdate_RoutesByLanguage -v
```

To exercise the CLI end-to-end against this repo itself: `go run ./cmd/kgraph build`, then e.g. `go run ./cmd/kgraph context <target>` / `query` / `explain` / `search` / `serve`. `--repo` (default `.`) and `--db` (default: per-user cache dir keyed by `--repo`) are global flags on every subcommand.

## Spec-driven workflow (openspec)

This repo uses `openspec/` as the source of truth for intended behavior, tracked separately from code:

- `openspec/specs/<capability>/spec.md` — current, accepted behavior per capability (one per major package/feature: code-graph-extraction, graph-storage, graph-model, incremental-update, build-pipeline, context-assembly, node-search, query-engine, entity-summarization, graph-analytics, graph-export, graph-visualization, server-api, mcp-server, kgraph-cli, extractor-interface, language-detection, multi-language-merge, edge-confidence, rationale-extraction, java-extraction, typescript-extraction, python-extraction).
- `openspec/changes/<change-id>/` — an in-flight or archived change: `proposal.md` (why/what), `design.md` (technical decisions/trade-offs), `tasks.md` (checklist tracking implementation progress). Archived changes live under `openspec/changes/archive/` (e.g. `archive/2026-09-13-multi-language-support/`).

Before making a non-trivial change, check whether an active change under `openspec/changes/` (outside `archive/`) already covers it — its `tasks.md` checkboxes reflect what's actually done vs. still pending, which is often more current than code comments or the README. When completing a task from an active change, update its `tasks.md` checkbox. If there's no active change and the work is non-trivial, consider proposing one before implementing.

## Architecture

```
cmd/kgraph/           CLI commands (cobra): build, update, context, search, query, path, explain,
                       prompt, analyze, export, report, mcp, summarize, serve, projects, prune
internal/analytics/    Graph analytics: node degree, "god node" detection, community detection
internal/build/        Pipeline wiring: parser -> graph -> store (build.Run, build.Update)
internal/context/      Token-budgeted context generation: BFS (bfs.go), rendering (render.go,
                        tokens.go), target resolution (target.go), plus query/path/explain/prompt
internal/enrich/       Edge confidence scoring (EXTRACTED vs INFERRED) and Rationale-comment extraction
internal/export/       graph.json export and Markdown report (GRAPH_REPORT.md) generation
internal/gitutil/      Git helpers (commit detection, changed-file diffing) for incremental update
internal/graph/        In-memory graph data structures (Node, Edge, NodeType, EdgeType, ID generators)
internal/mcp/          MCP server exposing the graph as tools (query_graph, get_node, shortest_path,
                        export_graph) over stdio or HTTP
internal/parser/       Multi-language extraction, factory-routed (see below)
internal/server/       HTTP server, SSE broadcaster, multi-project hub, staleness poller
internal/store/        SQLite persistence (modernc.org/sqlite, no cgo) — graph, summaries, build metadata
internal/summarizer/   Summarization pipeline: export pending nodes, apply provider results, lexical search
```

### Parser: factory-routed multi-language extraction

`internal/parser/` is a multi-language system. Structure:

- `internal/parser/common/` — the shared contract: `Extractor` interface (`ExtractRepo`, `ExtractPackages`, `FileExtensions`, `Language`), `LanguageDetector` (marker-file detection: `go.mod`→Go, `pom.xml`/`build.gradle`(`.kts`)→Java, `tsconfig.json`→TypeScript, `package.json` (no tsconfig)→JavaScript, `pyproject.toml`/`setup.py`/`requirements.txt`→Python, falling back to file-extension counting), `ExtractorFactory` (registry + routing), and `MergeGraphs`/`MergeGraphsWithWarnings` for combining per-language subgraphs in multi-language repos.
- `internal/parser/go/` — the original Go extractor (`go/ast`, `go/types`, `golang.org/x/tools/go/packages`). This is the most mature parser: full AST-based extraction of packages, structs, interfaces, functions, fields, calls, embeds, ORM tables/columns, and SQL migrations.
- `internal/parser/java/`, `internal/parser/typescript/`, `internal/parser/python/` — regex-based, best-effort extractors (not AST-based; there is no tree-sitter or other parsing library wired in for these languages). Treat extraction accuracy/completeness from these as lower-fidelity than the Go parser, especially for cross-file call resolution: no type information is available, so calls and framework-injected dependencies are only linked when a same-named node already exists in the graph, and unresolved targets are skipped silently.
- `internal/parser/parser.go` — the package-level entry point (`ExtractRepo`, `ExtractPackages`) that detects language(s), routes through the factory, runs each matched extractor, and merges results. `defaultFactory` is initialized in `init()` with all four extractors registered.

`internal/build/update.go` performs the same detect-route-merge dance for incremental updates, additionally filtering "known internal" packages per language so cross-language repos update correctly without reprocessing everything.

### Graph model

Nodes (`internal/graph/node.go`) and edges (`internal/graph/edge.go`) carry a `Type` plus a free-form `Properties map[string]any` for type-specific attributes (stored as JSON columns in SQLite — see `internal/store/schema.go`). Node types span Go-native concepts (`Package`, `Struct`, `Interface`, `Function`, `Field`, `Table`, `Column`, `Endpoint`, `ExternalDependency`), multi-language additions (`Enum`, `Decorator`, `Variable`, `TypeAlias`), and `Rationale` (an extracted `NOTE`/`WHY`/`HACK`/`TODO`/`FIXME`/`WARNING` comment or docstring, linked to the code it explains). Edge types mix Go-native relationships (`imports`, `calls`, `embeds`, `implements`, `has_method`, `has_field`, `maps_to_table`, `references_fk`, `reads_table`, `writes_table`, `exposes_endpoint`) with framework-oriented additions (`extends`, `injected`, `decorated`, `routed`) and `explains` (linking a `Rationale` node to its target). Every edge also carries a `Confidence` of `EXTRACTED` (read directly from source) or `INFERRED` (resolved via cross-file/heuristic analysis) — see `internal/enrich/confidence.go`, and surfaced via `kgraph path`/`kgraph explain`.

### Storage and caching

`internal/store` persists to SQLite via `modernc.org/sqlite` (pure Go, no cgo — deliberate, per `openspec/changes/archive/*/design.md`, to keep `kgraph`'s own build cgo-free). The database lives outside the analyzed repo by default, in a per-user cache directory keyed by a SHA-256 hash of the repo's absolute path (`internal/store/paths.go: DefaultDBPath`); `--db` overrides this. `internal/server`'s hub mode (`kgraph serve`) discovers every cached project via `internal/store/discover.go` and serves a picker alongside per-project live viewers.

### Context generation and derived views

`internal/context` resolves a target (file path, struct/function name, or search-query fallback — `target.go`), does a bounded BFS over persisted edges (`bfs.go`), and renders the result within a token budget (`render.go`, `tokens.go`) — the mechanism behind `kgraph context`, `kgraph query` (natural-language target resolution), `kgraph path` (shortest weighted path between two nodes), `kgraph explain` (summary + relations + rationale + analytics for one node), and `kgraph prompt` (renders context as an LLM-ready markdown prompt).

`internal/analytics` computes node degree, flags high-fan-in/out "god nodes", and detects/labels communities (`kgraph analyze`); `internal/export` dumps the graph as `graph.json` and generates `GRAPH_REPORT.md` (`kgraph export`, `kgraph report`); `internal/mcp` exposes the graph over MCP for AI assistants (`kgraph mcp`).
