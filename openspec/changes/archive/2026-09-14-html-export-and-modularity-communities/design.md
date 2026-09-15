## Context

Two independent subsystems are changing:

1. **Visualization** currently only exists live: `internal/server/static/{index.html,app.css,app.js}` is a vanilla-JS/canvas, no-build-step force-directed viewer, embedded into the `internal/server` binary via `go:embed` and served by `kgraph serve` against a live SQLite connection (fetch calls to `/api/graph`, `/api/graph/local`, `/api/node`, `/api/search`, plus an `EventSource` for live refresh). `internal/export` currently produces `graph.json` (the full enriched graph, already assembled once per export in `export.ToJSON`) and `GRAPH_REPORT.md`, both static files, but nothing visual.
2. **Community detection** (`internal/analytics/community.go`) is a hand-rolled label-propagation heuristic with an ad hoc small-community merge pass. Two prior follow-up changes already had to correct the historical record because label propagation was shipped where the original design.md promised "modularity-based refinement" — the gap between promise and implementation is long-standing, not new.

Both changes are additive/replacement work with no schema migration: node properties (`community`, `community_label`, `degree`, `god_node`) and the `DetectCommunities(g *graph.Graph, resolution float64)` signature are the stable contract every consumer (`graph-export`, the server's `/api/analytics/*`, `kgraph report`) already depends on; nothing about that contract changes shape, only what algorithm fills it in.

## Goals / Non-Goals

**Goals:**
- `kgraph export` produces a `graph.html` a person can open directly from disk (no `kgraph serve`, no SQLite) and get the same force-directed exploration experience: global graph, type filters, hover highlighting, local graph mode, node detail, search-to-focus, zoom/pan.
- One set of viewer assets serves both `kgraph serve` and `kgraph export`'s HTML output — not two forks that drift.
- Community detection maximizes modularity (Louvain) instead of using an ad hoc heuristic, while keeping `resolution`'s existing direction (higher → more, smaller communities) and the existing node-property contract.

**Non-Goals:**
- Leiden (Louvain's refinement-phase successor). It fixes a real Louvain weakness (occasional badly-connected communities) but needs a non-trivial extra phase; typical code-repo graphs are small enough that plain Louvain's output quality is very unlikely to be the bottleneck. Revisit only if dogfooding on large repos shows a real problem.
- Exact client-side parity with `SearchNodes`' Go-side ranking for the static viewer's search box. Re-implementing identical ranking twice (Go and JS) from the same spec is a maintenance trap; static-mode search is a reasonable client-side approximation, not a guaranteed-identical result set.
- Any change to `kgraph serve`'s live behavior. The static export is a new artifact; the live viewer's requirements (`graph-visualization`) are unchanged.
- Enforcing a size ceiling on `graph.html` for very large graphs. Noted as a risk below, not solved here.

## Decisions

### Decision 1: Extract viewer assets into a shared package, reused by server and export
Move `internal/server/static/{index.html,app.css,app.js}` and the `go:embed` + bootstrap-injection logic (`static.go`'s `staticFS`, `indexTemplate`, `renderIndex`'s templating half) into a new `internal/viewer` package. `internal/server` keeps the HTTP-specific half (route wiring, `staticAssetHandler`, SSE) and calls into `internal/viewer` for the embedded files and the template-execution helper. `internal/export` imports the same package for its `ToHTML`.

Alternative considered: let `internal/export` import `internal/server` directly. Rejected — `internal/server` pulls in hub discovery, the SSE broadcaster, and the staleness poller, none of which `export` needs; the dependency would be backwards (a batch/CLI concern depending on a long-running-server concern) and the reverse-engineered coupling more likely to bit-rot.

### Decision 2: `graph.html` is a single self-contained file, not a directory of assets
`ToHTML` inlines `app.css` into a `<style>` block and `app.js` into a `<script>` block (string substitution on the existing template, replacing the `<link rel=stylesheet>`/`<script src>` tags), plus a `window.KGRAPH` bootstrap object carrying `static: true` and the full embedded dataset (the same `export.Graph` structure `ToJSON` already builds — reused directly, not re-derived).

Alternative considered: ship `graph.html` + a sibling `graph-data.json` + copies of `app.css`/`app.js`, mirroring how the live server serves them. Rejected — the whole point is a single artifact someone can email, Slack, or drop in a wiki; a multi-file bundle reintroduces the "must stay together" fragility Graphify's single `graph.html` avoids.

### Decision 3: `app.js` gains a static data-source mode, not a separate script
Introduce a small data-access seam — `loadGlobalData(types)`, `loadLocalData(id, hops)`, `getNodeDetail(id)`, `searchNodes(q)` — with two implementations selected by `config.static`:
- **remote** (existing behavior): the current `fetchJSON` calls against `apiBase`.
- **static** (new): operates entirely on `window.KGRAPH.data`, computing local-graph hop expansion via an in-browser BFS over the embedded edge list (the same shape of traversal `internal/context/bfs.go` does server-side, just re-expressed in JS since there's no server to call) and search via a simple in-browser scoring function.

Everything downstream of that seam — rendering, the physics simulation, filters, hover/selection, zoom/pan, the detail panel — already operates on in-memory JS objects and needs no change regardless of where the data came from.

Live-refresh (`EventSource`) is simply never initialized in static mode; there is no live source to subscribe to.

### Decision 4: Louvain replaces label propagation, keeping the existing signature and resolution direction
`DetectCommunities(g *graph.Graph, resolution float64)` is reimplemented as standard Louvain: repeated local moves of each node into whichever neighboring community most increases modularity (using `resolution` as Louvain's own resolution parameter, γ, which already has the same "higher γ → smaller communities" direction the current doc comment promises), then graph aggregation (each community collapses into one node, edges reweighted), repeated until no further gain. Node iteration order is fixed (sorted by ID) each pass, and a tie in modularity gain prefers, in order: staying in the current community, then the lowest community ID — the same tie-break philosophy label propagation already used, so output remains deterministic given the same graph and resolution.

`mergeSmallCommunities`'s ad hoc post-process is dropped rather than ported: Louvain's modularity objective already discourages the stray-singleton problem that step existed to patch. If dogfooding (`kgraph analyze` against this repo and a couple of others) shows otherwise, a similar merge pass can be reintroduced as a follow-up — it's cheap to add back, and premature to carry across the rewrite unverified.

`renumberCommunities` (deterministic small-integer IDs, densest-first) and `LabelCommunities` (community labeling from path/name tokens) are unchanged — they operate on whatever partition `DetectCommunities` produces and don't care which algorithm produced it.

Alternative considered: Leiden — see Non-Goals.

## Risks / Trade-offs

- **[Risk]** Static-mode search diverges from `SearchNodes`' server-side ranking → the same query can return a different order (or set) of results in `graph.html` vs. `kgraph serve`. → **Mitigation**: documented as an accepted approximation (Non-Goals); if it proves confusing in practice, a future change could factor the ranking into a portable/shared form.
- **[Risk]** Large graphs produce a large embedded dataset, bloating `graph.html` and slowing initial browser load/parse. → **Mitigation**: none enforced in this change (Non-Goals); revisit with real numbers from dogfooding before adding complexity like size warnings or trimming.
- **[Risk]** Extracting `internal/viewer` out of `internal/server` regresses the live viewer if embed paths or bootstrap injection break in the move. → **Mitigation**: mechanical extraction (move files verbatim, update imports/call sites only); existing `internal/server` tests (`hub_test.go`, `api_test.go`, `api_query_test.go`) already exercise the live viewer end-to-end and should catch breakage; add direct tests for the new package.
- **[Risk]** Louvain changes community assignments for every graph that's already been built — anyone relying on specific community IDs/groupings from before this change sees different ones after. → **Mitigation**: called out as breaking in the proposal; community IDs were never a stable contract across rebuilds/reclusters even before this change (label propagation itself wasn't guaranteed stable under, say, added edges), so this is a quality-direction change, not a new category of instability.
- **[Risk]** Modularity optimization does more work per pass than label propagation. → **Mitigation**: code-repo graphs are small (hundreds to low thousands of nodes in practice); Louvain is near-linear in practice; if it ever matters, an iteration cap analogous to today's `maxLabelPropagationIterations` is a one-line addition.

## Migration Plan

No schema or data migration: community/degree/god-node values are recomputed from scratch on every `kgraph build`/`kgraph update`/`kgraph analyze --recluster`, never diffed against prior stored values. The two subsystems in this change are independent and can ship/rollback separately:
- The HTML-export path is purely additive (a new file written by `kgraph export`); rolling it back means reverting `internal/export/html.go`, the `internal/viewer` extraction, and the `cmd_export.go` call site, with no effect on `graph.json`/`GRAPH_REPORT.md`.
- The Louvain swap is a drop-in replacement behind the existing `DetectCommunities` signature; rolling it back means reverting `internal/analytics/community.go` to label propagation with no effect on the export/viewer work.

## Open Questions

- Should `kgraph export` gain a `--no-html` (or similar) escape hatch for people who don't want the extra file, e.g. on very large graphs? Leaning no for v1, consistent with the existing "export always writes both artifacts" precedent from the prior `graph-export` conformance follow-up — revisit if dogfooding shows a real need.
- Is Louvain's plain local-moving pass, without `mergeSmallCommunities`, actually free of stray singletons on real code graphs, or does that safety net need to come back? To be settled empirically during implementation (dogfood against this repo and at least one larger multi-language repo) rather than guessed here.
