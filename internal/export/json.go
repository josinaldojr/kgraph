// Package export serializes an enriched, analyzed graph for consumption
// outside kgraph: a portable graph.json (design.md Decision 9) and a
// generated GRAPH_REPORT.md summary (Decision 10).
package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// Node is graph.Node's exported (JSON) shape, with its stored summary
// (if any) and any linked rationale attached — the graph is the single
// source of truth, so export is just serialization, per design.md
// Decision 9.
type Node struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	File       string          `json:"file,omitempty"`
	LineStart  int             `json:"line_start,omitempty"`
	LineEnd    int             `json:"line_end,omitempty"`
	Signature  string          `json:"signature,omitempty"`
	Hash       string          `json:"hash"`
	Properties map[string]any  `json:"properties,omitempty"`
	Summary    string          `json:"summary,omitempty"`
	Rationale  []RationaleItem `json:"rationale,omitempty"`
}

// RationaleItem is one Rationale node linked to an exported Node via an
// `explains` edge.
type RationaleItem struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Edge is graph.Edge's exported (JSON) shape.
type Edge struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	SrcID      string         `json:"src_id"`
	DstID      string         `json:"dst_id"`
	Confidence string         `json:"confidence"`
	Properties map[string]any `json:"properties,omitempty"`
}

// Community is one entry in graph.json's top-level communities array,
// aggregating what analytics already assigned to each member node.
type Community struct {
	ID      int      `json:"id"`
	Label   string   `json:"label"`
	NodeIDs []string `json:"node_ids"`
}

// Metadata is graph.json's top-level metadata object, per graph-export's
// "Full graph export" scenario.
type Metadata struct {
	RepoPath  string   `json:"repo_path"`
	BuiltAt   string   `json:"built_at,omitempty"`
	NodeCount int      `json:"node_count"`
	EdgeCount int      `json:"edge_count"`
	Languages []string `json:"languages,omitempty"`
}

// Graph is the top-level graph.json document.
type Graph struct {
	Nodes       []Node      `json:"nodes"`
	Edges       []Edge      `json:"edges"`
	NodeCount   int         `json:"node_count"`
	EdgeCount   int         `json:"edge_count"`
	Communities []Community `json:"communities"`
	GodNodes    []string    `json:"god_nodes"`
	Metadata    Metadata    `json:"metadata"`
}

// ToJSON serializes g (nodes with all Properties, their stored summaries,
// and linked rationale; edges with Confidence; aggregate communities and
// god-nodes; and repo-level metadata) as indented JSON, per graph-export's
// "Full graph export" and "Export includes rationale" scenarios — the
// full graph, not a subset. repoPath identifies the build in build_meta
// (for metadata.built_at) and is echoed as metadata.repo_path.
func ToJSON(g *graph.Graph, s *store.Store, repoPath string) ([]byte, error) {
	summaries, err := s.SummariesByLevel(summarizer.LevelNode)
	if err != nil {
		return nil, fmt.Errorf("export: loading summaries: %w", err)
	}

	rationaleByTarget := rationaleIndex(g)

	doc := Graph{NodeCount: g.NodeCount(), EdgeCount: g.EdgeCount()}
	for _, n := range g.Nodes() {
		summary := ""
		if rec, ok := summaries[n.ID]; ok {
			summary = rec.Summary
		}
		doc.Nodes = append(doc.Nodes, Node{
			ID: n.ID, Type: string(n.Type), File: n.File,
			LineStart: n.LineStart, LineEnd: n.LineEnd,
			Signature: n.Signature, Hash: n.Hash,
			Properties: n.Properties, Summary: summary,
			Rationale: rationaleByTarget[n.ID],
		})
	}
	for _, e := range g.Edges() {
		doc.Edges = append(doc.Edges, Edge{
			ID: e.ID, Type: string(e.Type), SrcID: e.SrcID, DstID: e.DstID,
			Confidence: e.Confidence, Properties: e.Properties,
		})
	}
	doc.Communities = exportCommunities(g)
	doc.GodNodes = godNodeIDs(g)
	doc.Metadata, err = buildMetadata(g, s, repoPath)
	if err != nil {
		return nil, err
	}

	sort.Slice(doc.Nodes, func(i, j int) bool { return doc.Nodes[i].ID < doc.Nodes[j].ID })
	sort.Slice(doc.Edges, func(i, j int) bool { return doc.Edges[i].ID < doc.Edges[j].ID })

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("export: marshaling graph.json: %w", err)
	}
	return out, nil
}

// rationaleIndex builds, in one pass over g.Edges(), a map from each
// explained node's ID to the RationaleItems linked to it via `explains`
// edges — avoiding an O(nodes × edges) InEdges scan per node.
func rationaleIndex(g *graph.Graph) map[string][]RationaleItem {
	out := make(map[string][]RationaleItem)
	for _, e := range g.Edges() {
		if e.Type != graph.EdgeTypeExplains {
			continue
		}
		src := g.Node(e.SrcID)
		if src == nil || src.Type != graph.NodeTypeRationale {
			continue
		}
		kind, _ := src.Properties["kind"].(string)
		text, _ := src.Properties["text"].(string)
		out[e.DstID] = append(out[e.DstID], RationaleItem{Kind: kind, Text: text})
	}
	for id := range out {
		sort.Slice(out[id], func(i, j int) bool {
			if out[id][i].Kind != out[id][j].Kind {
				return out[id][i].Kind < out[id][j].Kind
			}
			return out[id][i].Text < out[id][j].Text
		})
	}
	return out
}

// exportCommunities projects g.Communities() (one member-node list per
// community ID) into graph.json's aggregate communities[] shape.
func exportCommunities(g *graph.Graph) []Community {
	byID := g.Communities()
	ids := make([]int, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	out := make([]Community, 0, len(ids))
	for _, id := range ids {
		members := byID[id]
		label := ""
		if len(members) > 0 {
			label = members[0].CommunityLabel()
		}
		nodeIDs := make([]string, len(members))
		for i, n := range members {
			nodeIDs[i] = n.ID
		}
		sort.Strings(nodeIDs)
		out = append(out, Community{ID: id, Label: label, NodeIDs: nodeIDs})
	}
	return out
}

// godNodeIDs returns every god node's ID, preserving g.GodNodes' descending
// degree order.
func godNodeIDs(g *graph.Graph) []string {
	nodes := g.GodNodes(-1)
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}

// buildMetadata assembles graph.json's metadata object: repoPath is echoed
// directly, built_at comes from build_meta (empty if no build is recorded
// for repoPath), and languages are the distinct Properties["language"]
// values already set by every parser.
func buildMetadata(g *graph.Graph, s *store.Store, repoPath string) (Metadata, error) {
	md := Metadata{RepoPath: repoPath, NodeCount: g.NodeCount(), EdgeCount: g.EdgeCount()}

	ts, ok, err := s.LastBuildAt(repoPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("export: loading build metadata: %w", err)
	}
	if ok {
		md.BuiltAt = time.Unix(ts, 0).UTC().Format(time.RFC3339)
	}

	seen := make(map[string]bool)
	for _, n := range g.Nodes() {
		lang, _ := n.Properties["language"].(string)
		if lang == "" || seen[lang] {
			continue
		}
		seen[lang] = true
		md.Languages = append(md.Languages, lang)
	}
	sort.Strings(md.Languages)

	return md, nil
}
