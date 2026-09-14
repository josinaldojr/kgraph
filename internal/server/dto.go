package server

// NodeSummaryDTO is one node as rendered in a graph view (global or local):
// enough to draw and label a circle, not the full detail panel.
type NodeSummaryDTO struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Signature string `json:"signature,omitempty"`
	File      string `json:"file,omitempty"`
	Degree    int    `json:"degree"`
}

// EdgeDTO is one edge as rendered in a graph view.
type EdgeDTO struct {
	Type       string `json:"type"`
	Src        string `json:"src"`
	Dst        string `json:"dst"`
	Confidence string `json:"confidence,omitempty"`
}

// GraphDTO is the payload for both /api/graph and /api/graph/local.
type GraphDTO struct {
	Nodes []NodeSummaryDTO `json:"nodes"`
	Edges []EdgeDTO        `json:"edges"`
}

// NoteDTO reports a node's note state, per graph-visualization's "Node
// detail view" requirement: current, stale (shown but flagged), or none.
type NoteDTO struct {
	State   string `json:"state"` // "current" | "stale" | "none"
	Summary string `json:"summary,omitempty"`
}

// RelationDTO is one edge from a node's perspective, for the detail panel.
type RelationDTO struct {
	EdgeType  string `json:"edge_type"`
	Direction string `json:"direction"` // "out" | "in"
	NodeID    string `json:"node_id"`
	NodeType  string `json:"node_type"`
}

// NodeDetailDTO is the full payload for /api/node.
type NodeDetailDTO struct {
	ID             string        `json:"id"`
	Type           string        `json:"type"`
	Signature      string        `json:"signature,omitempty"`
	File           string        `json:"file,omitempty"`
	LineStart      int           `json:"line_start,omitempty"`
	LineEnd        int           `json:"line_end,omitempty"`
	Note           NoteDTO       `json:"note"`
	FileNote       *NoteDTO      `json:"file_note,omitempty"`
	Relations      []RelationDTO `json:"relations"`
	Degree         int           `json:"degree"`
	GodNode        bool          `json:"god_node"`
	Community      int           `json:"community"`
	CommunityLabel string        `json:"community_label,omitempty"`
}

// SearchResultDTO is one ranked result from /api/search.
type SearchResultDTO struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	File  string `json:"file,omitempty"`
	Score int    `json:"score"`
}

// QueryDTO is the payload for /api/query: the natural-language question
// and its rendered, token-budgeted subgraph answer (the same text
// `kgraph query` prints), per query-engine's query-as-API requirement.
type QueryDTO struct {
	Question string `json:"question"`
	Result   string `json:"result"`
}

// PathHopDTO is one hop in a PathDTO: the node reached, and — except for
// the first hop — the edge type and confidence that reached it.
type PathHopDTO struct {
	NodeID     string `json:"node_id"`
	NodeType   string `json:"node_type"`
	ViaEdge    string `json:"via_edge,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// PathDTO is the payload for /api/path: the shortest weighted path found
// between two nodes (see context.Path), or an empty Hops with Found=false
// if none exists.
type PathDTO struct {
	Found bool         `json:"found"`
	Hops  []PathHopDTO `json:"hops"`
}

// ExplainDTO is the payload for /api/explain: a node's full-detail
// rendering (summary, relations with confidence, rationale, analytics
// metadata — the same text `kgraph explain` prints).
type ExplainDTO struct {
	NodeID string `json:"node_id"`
	Result string `json:"result"`
}
