## 1. Export: complete graph.json

- [x] 1.1 Add `Communities []Community`, `GodNodes []string`, and `Metadata Metadata` types/fields to the `Graph` doc struct in `internal/export/json.go`; add `Rationale []Rationale` to `Node`
- [x] 1.2 Define `Community{ID int, Label string, NodeIDs []string}` and `Rationale{Kind, Text string}` structs
- [x] 1.3 Change `ToJSON` signature to `ToJSON(g *graph.Graph, s *store.Store, repoPath string) ([]byte, error)`
- [x] 1.4 Populate `Metadata{RepoPath, BuiltAt, NodeCount, EdgeCount, Languages}` — `BuiltAt` from `s.LastBuildAt(repoPath)` (RFC3339-formatted, empty if not found), `Languages` from the distinct `Properties["language"]` values across `g.Nodes()` (sorted)
- [x] 1.5 Populate `communities[]` from `g.Communities()`: one entry per community ID, label from any member's `CommunityLabel()`, `node_ids` sorted
- [x] 1.6 Populate `god_nodes[]` from `g.GodNodes(-1)` (IDs only, already sorted by degree)
- [x] 1.7 Precompute a `dstID -> []*graph.Node` (rationale nodes) index in one pass over `g.Edges()` (edges of type `explains`), then populate each exported node's `rationale[]` from it — no per-node `InEdges` re-scan
- [x] 1.8 Update `cmd/kgraph/cmd_export.go`'s call site for the new `ToJSON` signature
- [x] 1.9 Update `internal/export/json_test.go`: assert `communities`, `god_nodes`, `metadata`, and per-node `rationale` are present and correctly populated

## 2. Export/report: directory-based --output

- [x] 2.1 Change `cmd/kgraph/cmd_export.go`: `--output` now names a directory (default `kgraph-out`); create it if missing; write `graph.json` and, via `export.GenerateReport`, `GRAPH_REPORT.md` into it
- [x] 2.2 Add `--output` to `cmd/kgraph/cmd_report.go` (default `kgraph-out`); create the directory if missing; write only `GRAPH_REPORT.md` into it (drop the old `<repo>/GRAPH_REPORT.md` default)
- [x] 2.3 Update `cmd/kgraph/cmd_newcommands_test.go`'s `TestExportCommandWritesGraphJSON` and `TestReportCommandWritesGraphReport` for the new directory semantics
- [x] 2.4 Update `README.md`'s `--output` documentation for `export`/`report`, if present (N/A — README doesn't document these commands/flags yet)

## 3. internal/graph unit tests

- [x] 3.1 `internal/graph/node_test.go`: typed getters `Community()`, `IsGodNode()`, `Degree()`, `CommunityLabel()` — present/absent/wrong-type Properties cases
- [x] 3.2 `internal/graph/graph_test.go`: `GodNodes(n)` (ordering by degree, `n < 0` returns all, `n` truncation), `NodesByCommunity(id)`, `Communities()` (nodes without a community excluded)
- [x] 3.3 `internal/graph/ids_test.go`: determinism of `PackageID`, `StructID`, `FunctionID` (plain function vs. method), `FieldID`, `TableID`, `ColumnID`, `EndpointID`, `ExternalDependencyID`, `EdgeID`, `EnumID`, `DecoratorID`, `RationaleID` — same inputs produce the same ID, different inputs produce different IDs
- [x] 3.4 `internal/graph/edge_test.go`: confidence constants (`ConfidenceExtracted`, `ConfidenceInferred`) round-trip through `AddEdge`/`OutEdges`/`InEdges`

## 4. Confidence annotation: full edge-type coverage

- [x] 4.1 Expand `internal/enrich/confidence_test.go` with one assertion per EXTRACTED edge type: `imports`, `calls`, `has_method`, `has_field`, `embeds`, `extends` (also added `references_fk`/`exposes_endpoint`, previously-untested EXTRACTED-by-fallback types)
- [x] 4.2 Expand it with one assertion per INFERRED edge type: `implements`, `maps_to_table`, `reads_table`, `writes_table` (also added `injected`/`decorated`/`routed`/`explains`, the remaining `inferredEdgeTypes` entries)
- [x] 4.3 Add a case for an edge type not in either list, asserting `AnnotateConfidence` leaves it unset or documents the fallback — added `references_fk`/`exposes_endpoint` (fall through to EXTRACTED) plus `TestAnnotateConfidenceCoversEveryEdgeType`, an exhaustive guard over every `EdgeType` constant

## 5. Verification

- [x] 5.1 `go build ./...`, `go vet ./...`
- [x] 5.2 `go test ./...` passes
- [x] 5.3 Manually run `kgraph export --repo . --output /tmp/kgraph-demo` against this repo and confirm both files land there with the new graph.json fields populated
