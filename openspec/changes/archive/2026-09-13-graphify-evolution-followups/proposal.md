## Why

A post-archive review of `kgraph-graphify-evolution` found that two implementations don't fully satisfy the `graph-export` spec that was synced from that change: `graph.json` omits several fields the spec requires, and `--output` doesn't behave as specified. It also flagged thin test coverage in two other areas touched by that change. This follow-up closes those gaps so the implementation matches the already-accepted specs, and hardens the weak test coverage.

## What Changes

- Complete `internal/export/json.go`'s `ToJSON()` to emit the fields `graph-export`'s "Full graph export" and "Export includes rationale" scenarios already require: a top-level `communities[]` array (id, label, node_ids), a top-level `god_nodes[]` array, a `metadata` object (repo_path, built_at, node_count, edge_count, languages), and a per-node `rationale[]` array (kind + text) sourced from `Rationale` nodes linked via `explains` edges.
- **BREAKING**: Change `kgraph export --output` to name a directory, per `graph-export`'s literal "Custom output directory" scenario (`kgraph export --output /tmp/graph-out` writes both `graph.json` and `GRAPH_REPORT.md` to `/tmp/graph-out/`): `export` now writes both artifacts into `<dir>/` (default `./kgraph-out/` when `--output` is omitted), instead of only `graph.json` to a file path. `kgraph report` keeps its standalone, markdown-only purpose but gains its own `--output <dir>` flag (same default) for users who only want the report, writing `<dir>/GRAPH_REPORT.md`. Today `export --output` names a single file (default `graph.json`) and `report` has no `--output` flag at all (always writes `<repo>/GRAPH_REPORT.md`).
- Add unit tests for `internal/graph/` (node.go, edge.go, graph.go, ids.go), which currently has none: typed getters (`Community()`, `IsGodNode()`, `Degree()`, `CommunityLabel()`), graph-level analytics methods (`GodNodes()`, `NodesByCommunity()`, `Communities()`), and ID-generation determinism.
- Expand `internal/enrich/confidence.go`'s `AnnotateConfidence` tests to explicitly cover every edge type from design.md's EXTRACTED/INFERRED lists (imports, calls, has_method, has_field, embeds, extends = EXTRACTED; implements, maps_to_table, reads_table, writes_table = INFERRED), not just a sample.

No new capabilities and no requirement-level spec changes: `graph-export`'s spec already describes the target behavior from the prior change; this change brings the implementation into conformance with it and improves test coverage elsewhere. `graph-model` and `edge-confidence`'s requirements are unchanged — only their test coverage grows.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

(none — `graph-export`'s existing requirements already describe the target behavior; this change is an implementation-conformance fix, not a requirement change)

## Impact

- **Code**: `internal/export/json.go` (ToJSON), `cmd/kgraph/cmd_export.go`, `cmd/kgraph/cmd_report.go`.
- **Tests**: new `internal/graph/*_test.go` files; expanded `internal/enrich/confidence_test.go`; updated `internal/export/json_test.go` for the new fields; updated `cmd/kgraph/cmd_newcommands_test.go` for the new `--output` directory semantics.
- **Breaking**: `kgraph export --output <path>` now means a directory, not a file, and `export` now always writes both `graph.json` and `GRAPH_REPORT.md`; existing scripts passing a file path (e.g. `--output graph.json`) will instead get a directory named `graph.json/` containing both files. `kgraph report`'s default output moves from `<repo>/GRAPH_REPORT.md` to `./kgraph-out/GRAPH_REPORT.md` (overridable with its new `--output` flag).
- **Dependencies**: none new.
