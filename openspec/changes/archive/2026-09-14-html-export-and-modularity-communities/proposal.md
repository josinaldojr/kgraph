## Why

Two gaps surfaced comparing kgraph to Graphify (see the exploration that preceded this change): kgraph's graph.json/GRAPH_REPORT.md export has no portable, self-contained visual counterpart to Graphify's `graph.html` — today the force-directed viewer only runs live against `kgraph serve` — and kgraph's community detection is a hand-rolled label-propagation heuristic where an earlier design intended (and Graphify itself uses) real modularity optimization. Both are closeable now without touching the still-open, undecided threads from that exploration (language breadth, git hooks, doc/media ingestion, git-committed graph.json).

## What Changes

- New: a self-contained `graph.html` artifact, generated as part of `kgraph export`, that renders the same force-directed viewer as `kgraph serve` (node radius by degree, color by type, hover/neighbor highlighting, type filtering, zoom/pan, node detail, search-to-focus, local graph mode) entirely from data embedded in the file — no server, no live SQLite connection, openable directly from disk.
  - Extract the viewer's static assets (`index.html`, `app.css`, `app.js`) out of `internal/server` into a shared package so both `kgraph serve` and the new export path use one asset set instead of two.
  - Add a "static mode" to the existing viewer JS: when the page has no `apiBase` to call, it reads an embedded full-graph dataset and answers global/local graph rendering, node detail, and search from that dataset in-browser instead of via `fetch`. Live-refresh (`EventSource`) is inapplicable in this mode and stays off.
- **BREAKING**: `kgraph export` now always writes `graph.html` alongside `graph.json` and `GRAPH_REPORT.md` in its output directory — anything that only expected two files in that directory will see a third.
- Replace `internal/analytics/community.go`'s label-propagation clustering with a real modularity-optimization algorithm (Louvain: repeated local moves to maximize modularity, then aggregate-and-repeat), keeping the existing `DetectCommunities(g *graph.Graph, resolution float64)` signature, the `resolution` direction (higher = more, smaller communities), and the node property contract (`community`, `community_label`) unchanged so `graph-export`, `graph-analytics`'s API/report consumers, and `kgraph analyze --recluster`/`--resolution` keep working.
  - Leiden (Louvain's better-connected-communities successor) is deliberately out of scope here: it needs a refinement phase Louvain doesn't, for a quality gain that mostly matters on graphs much larger than typical code repositories — worth a future change if larger graphs make it necessary, not this one.

## Capabilities

### New Capabilities
- `graph-html-export`: a portable, self-contained `graph.html` file — no server or database required to view it — rendering the same force-directed graph as the live viewer, produced by `kgraph export`.

### Modified Capabilities
- `graph-export`: `kgraph export` SHALL also produce `graph.html` in its output directory, in addition to the existing `graph.json` and `GRAPH_REPORT.md`.
- `graph-analytics`: the community-detection requirement SHALL specify modularity-optimization (Louvain) clustering rather than an unspecified "heuristic clustering algorithm," including how `resolution` maps to Louvain's resolution parameter and what determinism guarantee replaces label propagation's tie-breaking rule.

## Impact

- **Code**: new shared static-asset package (extracted from `internal/server/static.go` and `internal/server/static/*`); `internal/export/` gains `html.go` (`ToHTML`); `cmd/kgraph/cmd_export.go` writes the third file; `internal/server/static/app.js` gains a static-data-source mode; `internal/analytics/community.go` is rewritten around Louvain; `internal/analytics/analyze.go` and `cmd/kgraph/cmd_analyze.go` are unaffected beyond the algorithm swap underneath `DetectCommunities`.
- **Tests**: `internal/analytics/community_test.go`'s determinism/resolution assertions need updating for Louvain's behavior; new tests for `ToHTML` and for the export command's third output file; new frontend-facing test coverage (or manual verification, given the existing viewer has no JS test harness) for static-mode rendering.
- **Docs**: README's Features/Usage sections gain `graph.html` under `kgraph export`.
- **Breaking**: `kgraph export`'s output directory gains a third file (`graph.html`) unconditionally; anything scripting against exactly two files there needs updating. Community IDs assigned by `kgraph analyze`/`kgraph build` will differ from prior label-propagation output after this change (community *membership* quality should improve, but specific ID/grouping values are not guaranteed stable across the algorithm swap).
- **Dependencies**: none new — Louvain is implemented in pure Go, matching the existing no-new-heavy-dependency constraint; the static viewer stays vanilla JS/canvas with no charting library, matching `internal/server/static/app.js`'s existing no-build-step design.
