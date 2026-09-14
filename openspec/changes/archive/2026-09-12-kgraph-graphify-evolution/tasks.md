## 1. Graph Model Extensions

- [x] 1.1 Add `Confidence string` field to `Edge` struct in `internal/graph/edge.go`
- [x] 1.2 Add confidence constants (`ConfidenceExtracted`, `ConfidenceInferred`) to `internal/graph/edge.go`
- [x] 1.3 Add `NodeTypeRationale` constant to `internal/graph/node.go`
- [x] 1.4 Add `EdgeTypeExplains` constant to `internal/graph/edge.go`
- [x] 1.5 Add `RationaleID(file, line, kind string) string` to `internal/graph/ids.go`
- [x] 1.6 Add typed getters to Node: `Community() int`, `IsGodNode() bool`, `Degree() int`, `CommunityLabel() string` in `internal/graph/node.go`
- [x] 1.7 Add `GodNodes(n int) []*Node`, `NodesByCommunity(id int) []*Node`, `Communities() map[int][]*Node` methods to Graph in `internal/graph/graph.go`
- [x] 1.8 Update `EdgeID()` to include confidence in deterministic ID or keep separate (design decision: keep separate, confidence doesn't affect identity)

## 2. Schema Migration

- [x] 2.1 Add `ALTER TABLE edges ADD COLUMN confidence TEXT NOT NULL DEFAULT 'EXTRACTED'` to schema in `internal/store/schema.go`
- [x] 2.2 Update `SaveGraph` in `internal/store/persist.go` to persist edge confidence
- [x] 2.3 Update `LoadGraph` in `internal/store/load.go` to read edge confidence
- [x] 2.4 Add migration test: opening old DB adds confidence column with default

## 3. Rationale Extraction

- [x] 3.1 Create `internal/enrich/` package
- [x] 3.2 Implement `ExtractRationale(g *graph.Graph, filePath, content string) []*graph.Node` in `internal/enrich/rationale.go` — extracts NOTE/WHY/HACK/TODO/FIXME/WARNING comments
- [x] 3.3 Implement `ExtractDocstrings(g *graph.Graph, filePath, content string) []*graph.Node` in `internal/enrich/rationale.go` — extracts Go doc comments, Python triple-quoted docstrings, JSDoc
- [x] 3.4 Implement `LinkRationale(g *graph.Graph, rationale []*graph.Node, target *graph.Node)` — creates `explains` edges from rationale to nearest code entity
- [x] 3.5 Implement `EnrichGraph(g *graph.Graph) ([]string, error)` in `internal/enrich/enrich.go` — orchestrates rationale extraction for all files in the graph
- [x] 3.6 Add rationale extraction to Go parser output (or as post-parse pass)
- [x] 3.7 Add rationale extraction for Python and TypeScript parsers
- [x] 3.8 Write unit tests for rationale extraction (NOTE in Go, WHY in Python, HACK in TypeScript)

## 4. Edge Confidence Annotation

- [x] 4.1 Implement `AnnotateConfidence(g *graph.Graph)` in `internal/enrich/confidence.go` — marks edges as EXTRACTED or INFERRED based on edge type and parser source
- [x] 4.2 Mark `imports`, `calls`, `has_method`, `has_field`, `embeds`, `extends` edges as EXTRACTED
- [x] 4.3 Mark `implements`, `maps_to_table`, `reads_table`, `writes_table` edges as INFERRED
- [x] 4.4 Integrate confidence annotation into `EnrichGraph()` pipeline
- [x] 4.5 Write unit tests for confidence annotation

## 5. Graph Analytics

- [x] 5.1 Create `internal/analytics/` package
- [x] 5.2 Implement `ComputeDegree(g *graph.Graph)` in `internal/analytics/degree.go` — sets Properties["degree"] on every node
- [x] 5.3 Implement `DetectGodNodes(g *graph.Graph, n int)` in `internal/analytics/god_nodes.go` — marks top-N by degree with Properties["god_node"] = true
- [x] 5.4 Implement community detection heuristic in `internal/analytics/community.go` — seed-based expansion with modularity refinement
- [x] 5.5 Implement `LabelCommunities(g *graph.Graph)` in `internal/analytics/community.go` — derives human-readable labels from node types/names in each community
- [x] 5.6 Implement `AnalyzeGraph(g *graph.Graph, godNodeCount int) error` in `internal/analytics/analyze.go` — orchestrates degree, god nodes, communities
- [x] 5.7 Write unit tests for degree computation
- [x] 5.8 Write unit tests for god-node detection
- [x] 5.9 Write unit tests for community detection (small synthetic graphs)

## 6. Build Pipeline Integration

- [x] 6.1 Update `internal/build/build.go` Run() to call enrich and analyze stages after parse
- [x] 6.2 Add `--no-analyze` flag to `kgraph build` command
- [x] 6.3 Update `internal/build/update.go` to re-run enrich and analyze after incremental re-extraction
- [x] 6.4 Add `kgraph analyze` command in `cmd/kgraph/cmd_analyze.go` — loads existing graph, runs analytics, saves back
- [x] 6.5 Add `--recluster` flag to `kgraph analyze`
- [x] 6.6 Add `--god-nodes` flag to `kgraph build` and `kgraph analyze`
- [x] 6.7 Write integration test: build produces nodes with analytics metadata

## 7. Context/Render Evolution

- [x] 7.1 Update `renderTargetSection` in `internal/context/render.go` to include rationale nodes linked via `explains` edges
- [x] 7.2 Update `renderRelatedNode` to include confidence tag on the connecting edge
- [x] 7.3 Update `renderContext` to include community label and god-node flag for target node
- [x] 7.4 Add community-aware relevance scoring to BFS expansion in `internal/context/bfs.go` — nodes in the same community as the target get a relevance boost
- [x] 7.5 Write tests for enriched render output

## 8. Query Engine

- [x] 8.1 Implement `Query(g *graph.Graph, s *store.Store, question string, maxTokens int) (string, error)` in `internal/context/query.go` — lexical search + BFS + community-scoped rendering
- [x] 8.2 Implement `Path(g *graph.Graph, srcID, dstID string) ([]PathHop, error)` in `internal/context/path.go` — Dijkstra with edge weights
- [x] 8.3 Implement `Explain(g *graph.Graph, s *store.Store, nodeID string) (string, error)` in `internal/context/explain.go` — full node context with rationale, community, confidence
- [x] 8.4 Implement `Prompt(g *graph.Graph, s *store.Store, nodeID string, maxTokens int) (string, error)` in `internal/context/prompt.go` — explain formatted as LLM-ready prompt with markdown sections
- [x] 8.5 Add `kgraph query "<question>"` command in `cmd/kgraph/cmd_query.go`
- [x] 8.6 Add `kgraph path <src> <dst>` command in `cmd/kgraph/cmd_path.go`
- [x] 8.7 Add `kgraph explain <node>` command in `cmd/kgraph/cmd_explain.go`
- [x] 8.8 Add `kgraph prompt <node>` command in `cmd/kgraph/cmd_prompt.go`
- [x] 8.9 Write tests for query (lexical relevance + community scoping)
- [x] 8.10 Write tests for path (Dijkstra with weighted edges)
- [x] 8.11 Write tests for explain (rationale, community, confidence in output)

## 9. Graph Export

- [x] 9.1 Create `internal/export/` package
- [x] 9.2 Implement `ToJSON(g *graph.Graph, s *store.Store) ([]byte, error)` in `internal/export/json.go` — serializes full graph with all metadata
- [x] 9.3 Implement `GenerateReport(g *graph.Graph, s *store.Store) (string, error)` in `internal/export/report.go` — generates GRAPH_REPORT.md
- [x] 9.4 Add `kgraph export` command in `cmd/kgraph/cmd_export.go` with `--output` flag
- [x] 9.5 Add `kgraph report` command in `cmd/kgraph/cmd_report.go`
- [x] 9.6 Write tests for JSON export (all fields present, valid JSON)
- [x] 9.7 Write tests for report generation (god nodes, communities, connections sections present)

## 10. Server API Extensions

- [x] 10.1 Add `QueryDTO`, `PathDTO`, `ExplainDTO` to `internal/server/dto.go`
- [x] 10.2 Add `handleQuery` handler in `internal/server/api.go` — wraps context.Query
- [x] 10.3 Add `handlePath` handler in `internal/server/api.go` — wraps context.Path
- [x] 10.4 Add `handleExplain` handler in `internal/server/api.go` — wraps context.Explain
- [x] 10.5 Update `NodeDetailDTO` to include degree, god_node, community, community_label
- [x] 10.6 Update `EdgeDTO` to include confidence field
- [x] 10.7 Register new routes in `internal/server/server.go` Handler()
- [x] 10.8 Register new routes in `internal/server/hub.go` Handler()
- [x] 10.9 Update `Snapshot` to include analytics metadata if needed
- [x] 10.10 Write tests for new API endpoints

## 11. MCP Server

- [x] 11.1 Create `internal/mcp/` package
- [x] 11.2 Add MCP Go SDK dependency (github.com/mark3labs/mcp-go)
- [x] 11.3 Implement MCP tool definitions in `internal/mcp/tools.go`: query_graph, get_node, shortest_path, export_graph
- [x] 11.4 Implement MCP server in `internal/mcp/server.go` — stdio and HTTP transport
- [x] 11.5 Add `kgraph mcp` command in `cmd/kgraph/cmd_mcp.go` with --transport and --port flags
- [x] 11.6 Add --api-key flag for HTTP transport authentication
- [x] 11.7 Write tests for MCP tool handlers

## 12. CLI Integration

- [x] 12.1 Register new commands in `cmd/kgraph/main.go`: query, path, explain, prompt, report, export, analyze, mcp
- [x] 12.2 Add --max-tokens flag to query and prompt commands
- [x] 12.3 Add --hops flag to query and explain commands
- [x] 12.4 Ensure all new commands work with --repo and --db global flags
- [x] 12.5 Write CLI integration tests for new commands
