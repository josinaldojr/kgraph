## Context

`internal/export/json.go`'s `ToJSON(g *graph.Graph, s *store.Store)` currently serializes only `nodes[]`/`edges[]`. `graph-export`'s spec (synced from the archived `kgraph-graphify-evolution` change) additionally requires a top-level `communities[]` array, a top-level `god_nodes[]` array, a `metadata` object, and a per-node `rationale[]` array. The graph already carries everything needed for this: `Node.Properties["community"]`/`["community_label"]`/`["god_node"]`/`["degree"]`/`["language"]` (analytics + parser), `graph.Graph.Communities()`/`GodNodes()` (analytics query methods), and `Rationale` nodes linked to code nodes via `explains` edges (enrich).

`cmd/kgraph/cmd_export.go` and `cmd/kgraph/cmd_report.go` also don't match the spec's "Export SHALL support output path configuration" requirement: `export --output` names a file, `report` has no `--output` at all.

## Goals / Non-Goals

**Goals:**
- Bring `ToJSON()` to full conformance with `graph-export`'s "Full graph export" and "Export includes rationale" scenarios.
- Make `kgraph export --output <dir>` write both `graph.json` and `GRAPH_REPORT.md` into `<dir>/`, matching the spec's literal "Custom output directory" scenario, defaulting to `./kgraph-out/`.
- Give `kgraph report` its own `--output <dir>` flag (same default) so it still works standalone.
- Add missing unit test coverage in `internal/graph/` and expand `internal/enrich/confidence_test.go`.

**Non-Goals:**
- No changes to `query`/`path`/`explain`/`prompt`/`analyze`/`mcp` behavior.
- No new analytics (communities/god-nodes computation logic is unchanged; this only exposes what's already computed).
- No change to `graph.json`'s existing `nodes[]`/`edges[]` shapes beyond adding the new `rationale` field to `Node`.

## Decisions

### Decision 1: `ToJSON` gains a `repoPath` parameter

**Choice**: `ToJSON(g *graph.Graph, s *store.Store, repoPath string) ([]byte, error)`.

**Rationale**: `metadata.repo_path` and `metadata.built_at` need the repo path — `built_at` comes from `store.LastBuildAt(repoPath)` (build_meta is keyed by repo path), and `repo_path` is just echoed. `metadata.languages` doesn't need it: every node already carries `Properties["language"]` (set by every language extractor via `common.AddLanguageProperty`), so languages are derived by collecting the distinct values across `g.Nodes()`.

**Alternatives considered**: Store `repoPath` in `build_meta` and have `ToJSON` look it up without a parameter — rejected because there's no reverse lookup (build_meta is keyed *by* repo_path); the caller (`cmd_export.go`) already has it from `resolvePaths()`, so passing it through is simplest.

### Decision 2: `communities[]` and `god_nodes[]` are derived, not stored separately

**Choice**: Build both arrays at export time from existing per-node properties: `communities[]` from `g.Communities()` (one entry per community: id, label from any member's `CommunityLabel()`, sorted `node_ids`), `god_nodes[]` from `g.GodNodes(-1)` (every node already flagged, sorted by degree descending).

**Rationale**: Analytics already computed and stored these per-node; the export layer's job is just to project them into the aggregate shape the spec wants. No new analytics computation.

### Decision 3: Per-node `rationale[]` is built by walking `explains` in-edges

**Choice**: For each exported node, scan `g.InEdges(n.ID)` for edges of `Type == graph.EdgeTypeExplains`, resolve `SrcID` to its `Rationale` node, and emit `{kind, text}` from that node's `Properties["kind"]`/`["text"]`. Omit the field (`omitempty`) when empty, so nodes without rationale don't bloat the JSON.

**Rationale**: This mirrors exactly how `internal/context/explain.go` already surfaces rationale — same edge direction, same property keys — so there's one source of truth for "how rationale attaches to a node," just reused here instead of reimplemented.

### Decision 4: `export` becomes the two-artifact bundle; `report` stays independent

**Choice**: `kgraph export --output <dir>` (default `./kgraph-out/`) always writes both `<dir>/graph.json` and `<dir>/GRAPH_REPORT.md`. `kgraph report --output <dir>` (same default) writes only `<dir>/GRAPH_REPORT.md`, for users who want just the markdown without the full graph dump.

**Rationale**: The spec's own scenario is unambiguous that a single `kgraph export --output X` invocation produces both files — that's the literal requirement, not a loose paraphrase. Keeping `report` as a thin, independent command (reusing the same `export.GenerateReport`) preserves its current single-purpose use (e.g., in a CI job that only wants the summary) without forcing a second, redundant flag design.

**Alternatives considered**:
- Only add `--output` to `report`, leave `export --output` as a file path: rejected, contradicts the spec scenario directly.
- Merge `report` into `export` (drop the standalone command): rejected as an unnecessary breaking removal beyond what the spec asks for.

### Decision 5: Default output directory is `./kgraph-out/`, resolved relative to the invocation's working directory

**Choice**: Both commands default `--output` to `kgraph-out` (a relative path), matching the spec's "Default output directory" scenario verbatim, rather than nesting it under `--repo`.

**Rationale**: The spec scenario shows `./kgraph-out/` with no repo-relative qualifier, and every other kgraph output (e.g. the old `GRAPH_REPORT.md` at `<repo>/GRAPH_REPORT.md`) already resolved relative to `--repo`; changing the default's anchor is a one-line flag default, called out explicitly here since it's easy to instead default it relative to `--repo` by mistake.

## Risks / Trade-offs

**[Risk] `--output` semantics change breaks existing scripts**
→ Anyone passing `--output graph.json` today gets a directory named `graph.json/` instead of a file. Mitigation: this is explicitly called out as **BREAKING** in the proposal; no compatibility shim is added since the flag's old behavior didn't match its own spec in the first place.

**[Risk] Large graphs make `rationale[]` lookups O(nodes × edges) if done naively**
→ Mitigation: precompute a `map[dstID][]*graph.Node` of explains-edges once per `ToJSON` call (single pass over `g.Edges()`), not a per-node `InEdges` scan repeated inside a loop that also re-scans; `internal/context/explain.go`'s existing per-target approach is fine for one node, but export touches every node, so a single precomputed index avoids O(N·E).

## Open Questions

None — the ambiguity in the spec's literal export-bundles-both-files scenario (Decision 4) is resolved in favor of the literal reading.
