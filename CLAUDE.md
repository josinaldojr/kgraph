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

To exercise the CLI end-to-end against this repo itself: `go run ./cmd/kgraph build`, then `go run ./cmd/kgraph context <target>` / `search` / `serve`.

## Spec-driven workflow (openspec)

This repo uses `openspec/` as the source of truth for intended behavior, tracked separately from code:

- `openspec/specs/<capability>/spec.md` — current, accepted behavior per capability (code-graph-extraction, graph-storage, incremental-update, context-assembly, node-search, entity-summarization, graph-visualization, kgraph-cli).
- `openspec/changes/<change-id>/` — an in-flight or archived change: `proposal.md` (why/what), `design.md` (technical decisions/trade-offs), `tasks.md` (checklist tracking implementation progress). Archived changes live under `openspec/changes/archive/`.

Before making a non-trivial change, check whether an active change under `openspec/changes/` already covers it (e.g. `openspec/changes/multi-language-support/`) — its `tasks.md` checkboxes reflect what's actually done vs. still pending, which is often more current than the code comments or README. When completing a task from an active change, update its `tasks.md` checkbox.

## Architecture

```
cmd/kgraph/          CLI commands (cobra): build, update, context, search, summarize, serve, projects, prune
internal/build/      Pipeline wiring: parser -> graph -> store (build.Run, build.Update)
internal/context/    Token-budgeted context generation (BFS over the graph from a target)
internal/gitutil/    Git helpers (commit detection, changed-file diffing) for incremental update
internal/graph/      In-memory graph data structures (Node, Edge, NodeType, EdgeType, ID generators)
internal/parser/     Multi-language extraction, factory-routed (see below)
internal/server/     HTTP server, SSE broadcaster, multi-project hub, staleness poller
internal/store/      SQLite persistence (modernc.org/sqlite, no cgo) — graph, summaries, build metadata
internal/summarizer/ Summarization pipeline: export pending nodes, apply provider results, lexical search
```

### Parser: factory-routed multi-language extraction

`internal/parser/` was refactored from a single Go-only parser into a multi-language system (see `openspec/changes/multi-language-support/`). Structure:

- `internal/parser/common/` — the shared contract: `Extractor` interface (`ExtractRepo`, `ExtractPackages`, `FileExtensions`, `Language`), `LanguageDetector` (marker-file detection: `go.mod`→Go, `pom.xml`/`build.gradle`→Java, `tsconfig.json`→TypeScript, `package.json`→JavaScript, `pyproject.toml`/`setup.py`/`requirements.txt`→Python, falling back to file-extension counting), `ExtractorFactory` (registry + routing), and `MergeGraphs`/`MergeGraphsWithWarnings` for combining per-language subgraphs in multi-language repos.
- `internal/parser/go/` — the original Go extractor (`go/ast`, `go/types`, `golang.org/x/tools/go/packages`). This is the most mature parser: full AST-based extraction of packages, structs, interfaces, functions, fields, calls, embeds, ORM tables/columns, and SQL migrations.
- `internal/parser/java/`, `internal/parser/typescript/`, `internal/parser/python/` — regex-based, best-effort extractors (NOT AST-based, despite `github.com/smacker/go-tree-sitter` being present in `go.mod` — tree-sitter is a planned but not-yet-wired dependency; see the comment atop `internal/parser/java/parser.go`). Treat extraction accuracy/completeness from these as lower-fidelity than the Go parser, and prefer completing their test coverage (fixtures/unit tests are the main gaps per `tasks.md` groups 4-6) over adding new best-effort heuristics.
- `internal/parser/parser.go` — the package-level entry point (`ExtractRepo`, `ExtractPackages`) that detects language(s), routes through the factory, runs each matched extractor, and merges results. `defaultFactory` is initialized in `init()` with all five extractors registered.

`internal/build/update.go` performs the same detect-route-merge dance for incremental updates, additionally filtering "known internal" packages per language so cross-language repos update correctly without reprocessing everything.

### Graph model

Nodes (`internal/graph/node.go`) and edges (`internal/graph/edge.go`) carry a `Type` plus a free-form `Properties map[string]any` for type-specific attributes (stored as JSON columns in SQLite — see `internal/store/schema.go`). Node types span both Go-native concepts (`Package`, `Struct`, `Interface`, `Function`, `Field`, `Table`, `Column`, `Endpoint`, `ExternalDependency`) and multi-language additions (`Enum`, `Decorator`, `Variable`, `TypeAlias`). Edge types likewise mix Go-native relationships (`imports`, `calls`, `embeds`, `implements`, `has_method`, `has_field`, `maps_to_table`, `references_fk`, `reads_table`, `writes_table`, `exposes_endpoint`) with framework-oriented additions (`extends`, `injected`, `decorated`, `routed`) used by the Java/TS/Python extractors for DI, decorators, and routing annotations.

### Storage and caching

`internal/store` persists to SQLite via `modernc.org/sqlite` (pure Go, no cgo — deliberate, per `openspec/changes/*/design.md`, to keep `kgraph`'s own build cgo-free). The database lives outside the analyzed repo by default, in a per-user cache directory keyed by a SHA-256 hash of the repo's absolute path (`internal/store/paths.go: DefaultDBPath`); `--db` overrides this. `internal/server`'s hub mode (`kgraph serve`) discovers every cached project via `internal/store/discover.go` and serves a picker alongside per-project live viewers.

### Context generation

`internal/context` resolves a target (file path, struct/function name, or search-query fallback — `target.go`), does a bounded BFS over persisted edges (`bfs.go`), and renders the result within a token budget (`render.go`, `tokens.go`) — this is the mechanism behind `kgraph context`.
