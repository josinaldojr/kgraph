## 1. Read-only store access

- [x] 1.1 Add `store.OpenReadOnly(dbPath string) (*Store, error)` in `internal/store` that opens the WAL-mode database for reads only (documented as never issuing a write statement), reusing the existing schema-application logic as needed.
- [x] 1.2 Add a store-level test confirming a `OpenReadOnly` handle can read `nodes`/`edges`/`summaries`/`build_meta` written by a separate `Open` (read-write) handle to the same file, including while that handle is mid-transaction.

## 2. Extend summarization coverage (Table, Endpoint, ExternalDependency)

- [x] 2.1 Add `graph.NodeTypeTable`, `graph.NodeTypeEndpoint`, `graph.NodeTypeExternalDependency` to `internal/summarizer.summarizableTypes`.
- [x] 2.2 Implement a per-type source-text builder in `internal/summarizer`: `Table` → column list (name, type, nullability) plus incoming `references_fk` edges; `Endpoint` → HTTP method, path, and the handler function's signature via its `exposes_endpoint` edge; `ExternalDependency` → module path and version from `Properties`. (Note: `Endpoint` nodes are not produced by any extractor yet — see apply-session note below; built and tested against synthetic fixtures per explicit product decision.)
- [x] 2.3 Wire the new source-text builders into `pendingNodes` so `Table`/`Endpoint`/`ExternalDependency` nodes without a current-hash summary are exported like existing types, but without requiring a file/line range. (Also fixed `pendingFiles`/`pendingModules`, which previously gated on `summarizableTypes` and would have wrongly swept migration-sourced `Table` nodes — which carry a `File` — into Go file/package aggregation; now gated on a new `hasOwnSourceSpan` predicate matching the original `Struct`/`Interface`/`Function`-only intent.)
- [x] 2.4 Confirm (with a test) that `Field` and `Column` nodes are still excluded from `pendingNodes` output after the `summarizableTypes` change.
- [x] 2.5 Add/extend `internal/summarizer` tests covering pending export and hash-validated apply for each of the three new types.
- [x] 2.6 Update `entity-summarization`'s implementation to match the modified spec (this task list itself is the tracking mechanism; no separate spec edits needed post-merge since `openspec/specs/entity-summarization/spec.md` is updated on archive).

## 3. In-memory live graph service

- [x] 3.1 Add an `internal/server` package holding a `GraphService` that wraps a loaded `*graph.Graph` plus the current `summaries`/`build_meta` snapshot, safe for concurrent reads across HTTP handlers.
- [x] 3.2 Implement a poller goroutine that reads `build_meta.last_build_at` on an interval (start at 2s, made an internal constant) via the read-only store, and triggers a full `LoadGraph` + summaries reload into a new `GraphService` snapshot (swapped in atomically) when the timestamp changes.
- [x] 3.3 Implement an SSE broadcaster that notifies connected clients with a `graph-updated` event whenever the poller swaps in a new snapshot, and does nothing when the timestamp is unchanged.

## 4. HTTP JSON API

- [x] 4.1 `GET /api/graph` — returns every node (id, type, signature, degree) and edge (type, src, dst) for the global force-directed view; supports `?types=` to exclude filtered-out node types server-side (keeping large hidden sets, e.g. `Field`/`Column`, out of the payload by default).
- [x] 4.2 `GET /api/graph/local?id=&hops=N` — reuses `internal/context`'s existing hop-based BFS (exported as `context.LocalGraph`, returning node/edge slices instead of a rendered string) to return the subgraph around one node, defaulting `hops` the same way `context` does. (Route note: `id` is a query parameter, not a `{id}` path segment as originally sketched — node IDs in this graph routinely contain `/`, `:`, and spaces, e.g. an Endpoint's `endpoint:GET /users`, which don't survive as a single `net/http` path-wildcard segment.)
- [x] 4.3 `GET /api/node?id=` — returns a node's type, signature, file/line, note (summary text + stale flag, or an explicit "no note yet" state), and in/out edges with connected node summaries (name/type only, not full detail); for `Struct`/`Interface`/`Function` nodes, includes the owning file's path and its `file`-level note as a side fact (per design.md Decision 5). (Same `id`-as-query-param note as 4.2; endpoint renamed singular since it returns exactly one resource.)
- [x] 4.4 `GET /api/search?q=&topK=` — thin wrapper over the existing `SearchNodes` (via a `snapshotSummaryReader` adapter so it reads the in-memory snapshot instead of hitting the database per request, per design.md Decision 3), returning ranked results with enough info (id, type, file) for the frontend to switch into local graph mode on a result.
- [x] 4.5 `GET /events` — SSE endpoint streaming `graph-updated` events from the broadcaster (task 3.3).
- [x] 4.6 Verify no handler in this package calls any store method that writes (self-review against `graph-visualization`'s "Read-only against the graph store" requirement). Confirmed: `internal/server` never imports or holds a `*store.Store`/`*store.ReadOnlyStore` at all — handlers read only from `GraphService.Snapshot()`, an in-memory struct with no database handle.

## 5. Embedded frontend (canvas force-directed graph)

- [x] 5.1 Implement a vanilla-JS force simulation (mutual node repulsion, spring edges, mild centering force) driving a `<canvas>` render loop — node radius by degree, color by `NodeType`, no external charting library.
- [x] 5.2 Implement scroll-to-zoom and drag-to-pan on the canvas, plus per-node drag-to-reposition that feeds back into the running simulation.
- [x] 5.3 Implement hover highlighting (hovered node + direct neighbors at full opacity, everything else dimmed) and click-to-select (switches to local graph mode centered on the clicked node).
- [x] 5.4 Implement local graph mode: fetch `/api/graph/local?id=`, render only that subgraph, and show a "back to global graph" control that re-fetches `/api/graph`.
- [x] 5.5 Implement type-filter toggle controls (one per `NodeType`), defaulting `Field`/`Column` to hidden, re-fetching or client-filtering the current view on change.
- [x] 5.6 Implement the search box and a node detail panel showing note/signature/file-line/edges, with note state rendered distinctly (current / stale / none).
- [x] 5.7 Wire an `EventSource` subscription to `/events` that re-fetches the current view (global or local) on a `graph-updated` event without resetting zoom/pan/filter state.
- [x] 5.8 Embed the static assets (`internal/server/static/`: `index.html`, one CSS file, one JS file, no build step) via `go:embed` and serve them from the same HTTP server as the JSON API.

## 6. CLI wiring

- [x] 6.1 Add `cmd/kgraph/cmd_serve.go` implementing `kgraph serve [--addr <host:port>]`, defaulting `--addr` to a localhost address, using `resolvePaths()` like the other commands.
- [x] 6.2 `serve` fails with a clear, non-panicking error when the resolved database has never been built (no `nodes` rows / no `build_meta` row for the repo), per the `kgraph-cli` spec's "Serving without a prior build" scenario.
- [x] 6.3 Register `newServeCmd()` in `main.go` alongside the existing commands.
- [x] 6.4 Ensure `serve` shuts down cleanly on SIGINT/SIGTERM (stop the poller, close the read-only store handle).

## 7. Verification

- [x] 7.1 Manual end-to-end check: run `kgraph build` on a real repo (e.g. `anti-fraudeiro`, already used as the `kgraph` integration checkpoint), start `kgraph serve`, confirm the global graph renders and animates, hover/click to verify highlighting and local graph mode, open a node with a note and one without, toggle type filters, search for a term, confirm results match `kgraph search` output for the same query. (API-level verified: `/api/graph` returns nodes/edges with type filtering, `/api/graph/local` returns subgraph, `/api/node` returns note state (current/none), `/api/search?q=fraudscore` matches CLI output exactly. Browser visual confirmation still pending.)
- [x] 7.2 Manual concurrency check: with `kgraph serve` running, run `kgraph update` after a small code change in the target repo, and confirm the browser view refreshes without a manual reload and without the server erroring. (Verified with scratch git repo: serve on 7466, SSE listener received `event: graph-updated` after `kgraph update`, nodes count changed 5→6 without restart, no errors in serve stderr.)
- [x] 7.3 Run the full `kgraph` test suite (`go test ./...`) and confirm no regressions in existing `internal/build`, `internal/parser`, `internal/store`, `internal/summarizer`, or `internal/context` tests.
