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
	Type string `json:"type"`
	Src  string `json:"src"`
	Dst  string `json:"dst"`
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
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Signature string        `json:"signature,omitempty"`
	File      string        `json:"file,omitempty"`
	LineStart int           `json:"line_start,omitempty"`
	LineEnd   int           `json:"line_end,omitempty"`
	Note      NoteDTO       `json:"note"`
	FileNote  *NoteDTO      `json:"file_note,omitempty"`
	Relations []RelationDTO `json:"relations"`
}

// SearchResultDTO is one ranked result from /api/search.
type SearchResultDTO struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	File  string `json:"file,omitempty"`
	Score int    `json:"score"`
}
