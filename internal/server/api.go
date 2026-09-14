package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/josinaldojr/kgraph/internal/context"
	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// hiddenByDefault are the node types excluded from /api/graph when no
// ?types= filter is given — the most numerous and least summarized types,
// per graph-visualization's "Type-based filtering" requirement's default.
var hiddenByDefault = map[graph.NodeType]bool{
	graph.NodeTypeField:  true,
	graph.NodeTypeColumn: true,
}

// Ceilings on client-controlled query parameters that drive BFS depth,
// result-set size, or rendered output size, per server-api's "Query
// parameter-driven work SHALL be bounded" requirement. An out-of-range
// value clamps to these rather than erroring — see design.md Decision 1.
const (
	MaxHops        = 6
	MaxTopK        = 100
	MaxQueryTokens = 20000
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v) // best-effort: client likely disconnected if this fails
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func degree(g *graph.Graph, id string) int {
	return len(g.OutEdges(id)) + len(g.InEdges(id))
}

func toNodeSummaryDTO(g *graph.Graph, n *graph.Node) NodeSummaryDTO {
	return NodeSummaryDTO{ID: n.ID, Type: string(n.Type), Signature: n.Signature, File: n.File, Degree: degree(g, n.ID)}
}

func toGraphDTO(g *graph.Graph, nodes []*graph.Node, edges []*graph.Edge) GraphDTO {
	dto := GraphDTO{Nodes: make([]NodeSummaryDTO, 0, len(nodes)), Edges: make([]EdgeDTO, 0, len(edges))}
	for _, n := range nodes {
		dto.Nodes = append(dto.Nodes, toNodeSummaryDTO(g, n))
	}
	for _, e := range edges {
		dto.Edges = append(dto.Edges, EdgeDTO{Type: string(e.Type), Src: e.SrcID, Dst: e.DstID, Confidence: e.Confidence})
	}
	sort.Slice(dto.Nodes, func(i, j int) bool { return dto.Nodes[i].ID < dto.Nodes[j].ID })
	sort.Slice(dto.Edges, func(i, j int) bool {
		if dto.Edges[i].Src != dto.Edges[j].Src {
			return dto.Edges[i].Src < dto.Edges[j].Src
		}
		return dto.Edges[i].Dst < dto.Edges[j].Dst
	})
	return dto
}

// allowedTypes parses ?types= into an allow-set: "" (not given) signals
// "apply hiddenByDefault", "all" signals "no filtering at all", and
// anything else is taken as an exact comma-separated allow-list.
func allowedTypes(raw string) (allow map[graph.NodeType]bool, all bool) {
	if raw == "" {
		return nil, false
	}
	if raw == "all" {
		return nil, true
	}
	allow = make(map[graph.NodeType]bool)
	for _, t := range strings.Split(raw, ",") {
		if t = strings.TrimSpace(t); t != "" {
			allow[graph.NodeType(t)] = true
		}
	}
	return allow, false
}

// handleGraph serves GET /api/graph?types= — the global force-directed
// view, per graph-visualization's "Force-directed graph rendering" and
// "Type-based filtering" requirements.
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	s.withSnapshot(w, r, apiGraph)
}

// withSnapshot resolves the Server's single pinned snapshot and hands it to
// fn, reporting 503 during the brief window before the poller's first load.
// Hub mode resolves snapshots differently (per project, via the runtime
// registry — see hub.go) but shares the same handler bodies below.
func (s *Server) withSnapshot(w http.ResponseWriter, r *http.Request, fn func(http.ResponseWriter, *http.Request, *Snapshot)) {
	snap := s.gs.Snapshot()
	if snap == nil {
		writeError(w, http.StatusServiceUnavailable, "graph not loaded yet")
		return
	}
	fn(w, r, snap)
}

// apiGraph implements the global graph view against a resolved snapshot,
// shared by single-project mode (/api/graph) and hub mode
// (/api/projects/<key>/graph).
func apiGraph(w http.ResponseWriter, r *http.Request, snap *Snapshot) {
	allow, all := allowedTypes(r.URL.Query().Get("types"))
	include := func(t graph.NodeType) bool {
		if all {
			return true
		}
		if allow != nil {
			return allow[t]
		}
		return !hiddenByDefault[t]
	}

	included := make(map[string]bool)
	var nodes []*graph.Node
	for _, n := range snap.Graph.Nodes() {
		if !include(n.Type) {
			continue
		}
		included[n.ID] = true
		nodes = append(nodes, n)
	}
	var edges []*graph.Edge
	for _, e := range snap.Graph.Edges() {
		if included[e.SrcID] && included[e.DstID] {
			edges = append(edges, e)
		}
	}
	writeJSON(w, toGraphDTO(snap.Graph, nodes, edges))
}

// handleGraphLocal serves GET /api/graph/local?id=&hops= — the local graph
// view centered on one node, per graph-visualization's "Local graph view"
// requirement. id is a query parameter, not a path segment: node IDs in
// this graph routinely contain '/', ':', and even spaces (e.g. an
// Endpoint's "endpoint:GET /users"), which don't survive as a single
// net/http path-wildcard segment.
func (s *Server) handleGraphLocal(w http.ResponseWriter, r *http.Request) {
	s.withSnapshot(w, r, apiGraphLocal)
}

// apiGraphLocal implements the local graph view against a resolved
// snapshot, shared by both serving modes.
func apiGraphLocal(w http.ResponseWriter, r *http.Request, snap *Snapshot) {
	id := r.URL.Query().Get("id")
	target := snap.Graph.Node(id)
	if target == nil {
		writeError(w, http.StatusNotFound, "no such node: "+id)
		return
	}
	hops := intParam(r, "hops", context.DefaultHops, MaxHops)
	nodes, edges := context.LocalGraph(snap.Graph, target, hops)
	writeJSON(w, toGraphDTO(snap.Graph, nodes, edges))
}

func noteFor(summaries map[string]store.SummaryRecord, id string) NoteDTO {
	rec, ok := summaries[id]
	if !ok || rec.Summary == "" {
		return NoteDTO{State: "none"}
	}
	if rec.Stale {
		return NoteDTO{State: "stale", Summary: rec.Summary}
	}
	return NoteDTO{State: "current", Summary: rec.Summary}
}

// hasOwnFileNote reports whether n's detail view should surface its owning
// file's aggregated note as a side fact, per design.md Decision 5 — only
// for node types with a file/line span within a Go source file
// (Struct/Interface/Function); Table/Endpoint/ExternalDependency have no
// such file-note concept.
func hasOwnFileNote(n *graph.Node) bool {
	switch n.Type {
	case graph.NodeTypeStruct, graph.NodeTypeInterface, graph.NodeTypeFunction:
		return n.File != ""
	default:
		return false
	}
}

// handleNode serves GET /api/node?id= — the detail panel payload, per
// graph-visualization's "Node detail view" requirement.
func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	s.withSnapshot(w, r, apiNode)
}

// apiNode implements the node detail view against a resolved snapshot,
// shared by both serving modes.
func apiNode(w http.ResponseWriter, r *http.Request, snap *Snapshot) {
	id := r.URL.Query().Get("id")
	n := snap.Graph.Node(id)
	if n == nil {
		writeError(w, http.StatusNotFound, "no such node: "+id)
		return
	}

	dto := NodeDetailDTO{
		ID: n.ID, Type: string(n.Type), Signature: n.Signature,
		File: n.File, LineStart: n.LineStart, LineEnd: n.LineEnd,
		Note:           noteFor(snap.NodeSummaries, n.ID),
		Degree:         n.Degree(),
		GodNode:        n.IsGodNode(),
		Community:      n.Community(),
		CommunityLabel: n.CommunityLabel(),
	}
	if hasOwnFileNote(n) {
		fn := noteFor(snap.FileSummaries, n.File)
		dto.FileNote = &fn
	}

	for _, e := range snap.Graph.OutEdges(n.ID) {
		if dst := snap.Graph.Node(e.DstID); dst != nil {
			dto.Relations = append(dto.Relations, RelationDTO{EdgeType: string(e.Type), Direction: "out", NodeID: dst.ID, NodeType: string(dst.Type)})
		}
	}
	for _, e := range snap.Graph.InEdges(n.ID) {
		if src := snap.Graph.Node(e.SrcID); src != nil {
			dto.Relations = append(dto.Relations, RelationDTO{EdgeType: string(e.Type), Direction: "in", NodeID: src.ID, NodeType: string(src.Type)})
		}
	}
	sort.Slice(dto.Relations, func(i, j int) bool {
		if dto.Relations[i].EdgeType != dto.Relations[j].EdgeType {
			return dto.Relations[i].EdgeType < dto.Relations[j].EdgeType
		}
		return dto.Relations[i].NodeID < dto.Relations[j].NodeID
	})
	writeJSON(w, dto)
}

// handleSearch serves GET /api/search?q=&topK= — a thin wrapper over the
// existing lexical SearchNodes, per graph-visualization's "Search-to-focus"
// requirement.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	s.withSnapshot(w, r, apiSearch)
}

// apiSearch implements search against a resolved snapshot, shared by both
// serving modes.
func apiSearch(w http.ResponseWriter, r *http.Request, snap *Snapshot) {
	q := r.URL.Query().Get("q")
	topK := intParam(r, "topK", 10, MaxTopK)
	results, err := summarizer.SearchNodes(snap.Graph, snapshotSummaryReader{snap}, q, topK)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]SearchResultDTO, 0, len(results))
	for _, res := range results {
		dtos = append(dtos, SearchResultDTO{ID: res.Node.ID, Type: string(res.Node.Type), File: res.Node.File, Score: res.Score})
	}
	writeJSON(w, dtos)
}

// handleQuery serves GET /api/query?q=&hops=&max_tokens= — a thin wrapper
// over context.Query, per query-engine's server-API extension requirement.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	s.withSnapshot(w, r, apiQuery)
}

// apiQuery implements query against a resolved snapshot, shared by both
// serving modes.
func apiQuery(w http.ResponseWriter, r *http.Request, snap *Snapshot) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: q")
		return
	}
	hops := intParam(r, "hops", context.DefaultHops, MaxHops)
	maxTokens := intParam(r, "max_tokens", context.DefaultMaxTokens, MaxQueryTokens)

	result, err := context.Query(snap.Graph, snapshotSummaryReader{snap}, q, hops, maxTokens)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, QueryDTO{Question: q, Result: result})
}

// handlePath serves GET /api/path?src=&dst= — a thin wrapper over
// context.Path.
func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
	s.withSnapshot(w, r, apiPath)
}

// apiPath implements path-finding against a resolved snapshot, shared by
// both serving modes.
func apiPath(w http.ResponseWriter, r *http.Request, snap *Snapshot) {
	src := r.URL.Query().Get("src")
	dst := r.URL.Query().Get("dst")
	if src == "" || dst == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameters: src, dst")
		return
	}

	hops, err := context.Path(snap.Graph, src, dst)
	if err != nil {
		writeJSON(w, PathDTO{Found: false})
		return
	}
	dtos := make([]PathHopDTO, len(hops))
	for i, h := range hops {
		dtos[i] = PathHopDTO{NodeID: h.Node.ID, NodeType: string(h.Node.Type), ViaEdge: string(h.ViaEdge), Confidence: h.Confidence}
	}
	writeJSON(w, PathDTO{Found: true, Hops: dtos})
}

// handleExplain serves GET /api/explain?id=&hops= — a thin wrapper over
// context.Explain.
func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	s.withSnapshot(w, r, apiExplain)
}

// apiExplain implements explain against a resolved snapshot, shared by
// both serving modes.
func apiExplain(w http.ResponseWriter, r *http.Request, snap *Snapshot) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: id")
		return
	}
	hops := intParam(r, "hops", 1, MaxHops)

	result, err := context.Explain(snap.Graph, snapshotSummaryReader{snap}, id, hops)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, ExplainDTO{NodeID: id, Result: result})
}

// intParam parses the named query parameter as a positive int, clamped to
// max, falling back to def if it's missing or invalid.
func intParam(r *http.Request, name string, def, max int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
