## 1. Louvain community detection

- [x] 1.1 Implement Louvain's local-moving phase in `internal/analytics/community.go`: sorted-ID node iteration, per-node modularity-gain evaluation across neighboring communities (using `resolution` as Louvain's γ), deterministic tie-break (current community first, then lowest community ID).
- [x] 1.2 Implement the aggregation phase (collapse each community into one node, reweight edges, repeat local-moving on the aggregated graph) and the repeat-until-no-gain outer loop, with an iteration cap mirroring the old `maxLabelPropagationIterations` for pathological-input safety.
- [x] 1.3 Remove `mergeSmallCommunities` and its call site; keep `renumberCommunities` and `LabelCommunities` unchanged, wired to Louvain's output.
- [x] 1.4 Update `internal/analytics/community.go`'s package comment / `DetectCommunities` doc comment to describe Louvain instead of label propagation.
- [x] 1.5 Update `internal/analytics/community_test.go`'s resolution/determinism assertions for Louvain's actual behavior; add a determinism test that runs detection twice on the same graph/resolution and asserts identical output, per the `graph-analytics` delta's new scenario.
- [x] 1.6 Dogfood: run `kgraph build && kgraph analyze --recluster` against this repo (and, if convenient, one other multi-language repo) and eyeball community quality/singleton counts; decide per design.md's open question whether a small-community merge safety net needs to come back. Result: at DefaultResolution (1.0), 1287 nodes formed 65 communities with only 4 singletons (0.3%) — no stray-singleton problem, so `mergeSmallCommunities` stays dropped. `--resolution` was verified to meaningfully reshape the partition (e.g. resolution=50 splits the dominant 871-node community down to a max of 151; resolution=200 yields 369 communities), confirming the knob works even though this repo's graph needs a much higher resolution than the bridged-cluster unit-test fixture to visibly fragment its one large, densely-interconnected core.

## 2. Shared viewer asset package

- [x] 2.1 Create `internal/viewer` (or similar name) containing the moved `static/{index.html,app.css,app.js}`, the `go:embed` directive, and the template-parsing/bootstrap-injection logic currently in `internal/server/static.go`.
- [x] 2.2 Update `internal/server` to consume `internal/viewer` for its embedded assets and index rendering, keeping `staticAssetHandler`/HTTP route wiring and SSE in `internal/server`.
- [x] 2.3 Run the existing `internal/server` test suite (`hub_test.go`, `api_test.go`, `api_query_test.go`, `api_bounds_test.go`, `recover_test.go`) to confirm the extraction didn't regress the live viewer; add direct tests for `internal/viewer`'s embed/template helpers if coverage is thin after the move.

## 3. Static-mode viewer JS

- [x] 3.1 Add a `config.static` bootstrap flag and an embedded full-dataset field (`window.KGRAPH.data`) alongside the existing bootstrap config shape.
- [x] 3.2 Introduce the data-access seam in `app.js` (`loadGlobalData`, `loadLocalData`, `getNodeDetail`, `searchNodes`) with the existing `fetchJSON`-based calls as the "remote" implementation.
- [x] 3.3 Implement the "static" implementation: type-filtered global graph and hop-bounded local-graph BFS over the embedded edge list, node detail lookup by ID, and a client-side search-scoring function.
- [x] 3.4 Gate `EventSource`/live-refresh initialization on `!config.static`.
- [x] 3.5 Manually verify in a browser (per this project's UI-testing norm, since there's no JS test harness): open a generated `graph.html` via `file://`, exercise global graph, type filtering, hover highlighting, drag, zoom/pan, node detail, search-to-focus, and local-graph mode with the "back to global" control. Verified via Chrome DevTools MCP against a `graph.html` generated from this repo (553 nodes/1168 edges): global graph rendered; toggling "Function" off/on correctly filtered to 138/189 nodes/edges and back; hovering a node dimmed all non-neighbors; search for "Louvain" returned the `louvain()` function plus its rationale comments, and clicking a result switched to local-graph mode (16 nodes/16 edges) and opened the correct detail panel (signature, file, 4 relations); node drag pulled connected neighbors via the spring simulation; background drag panned the view; wheel zoomed anchored on the cursor; "Back to global graph" returned to the full 553/1168 view. No JS errors in the console and no live network requests (the one console message present — a benign `file:` frame-origin notice from the page's own initial load — predates any interaction and isn't caused by app code).

## 4. HTML export artifact

- [x] 4.1 Add `internal/export/html.go`'s `ToHTML(g *graph.Graph, s *store.Store, repoPath string) ([]byte, error)`, reusing `export.ToJSON`'s already-assembled `Graph` document as the embedded dataset and `internal/viewer`'s template for markup, inlining CSS/JS into one self-contained file per design.md Decision 2.
- [x] 4.2 Wire `cmd/kgraph/cmd_export.go` to write `graph.html` into the output directory alongside `graph.json` and `GRAPH_REPORT.md`, updating its success message to mention all three files.
- [x] 4.3 Add `internal/export/html_test.go` covering: valid HTML output, embedded dataset matches `ToJSON`'s content, `static: true` bootstrap flag set, no reference to any external `/api/...` or `/static/...` URL in the output.
- [x] 4.4 Update `cmd/kgraph/cmd_newcommands_test.go` (or wherever `export`'s output-directory behavior is tested) to assert the third file's presence.

## 5. Docs

- [x] 5.1 Update README's Features list and `kgraph export` usage section to mention `graph.html`.
- [x] 5.2 Update README's community-detection description (if any) to reflect Louvain instead of label propagation.
