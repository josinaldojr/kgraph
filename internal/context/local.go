package context

import "github.com/josinaldojr/kgraph/internal/graph"

// LocalGraph returns the node set discovered within hops edges of target —
// via the same BFS expandSubgraph performs for GetContext — plus every
// edge in g whose endpoints are both in that discovered set (the full
// induced subgraph, not just the BFS tree edges used for discovery). This
// is what internal/server's local graph view (Obsidian-style, centered on
// one node) renders; hops <= 0 uses DefaultHops, matching GetContext.
func LocalGraph(g *graph.Graph, target *graph.Node, hops int) ([]*graph.Node, []*graph.Edge) {
	if hops <= 0 {
		hops = DefaultHops
	}
	found := expandSubgraph(g, target, hops)

	nodes := make([]*graph.Node, 0, len(found))
	for _, d := range found {
		nodes = append(nodes, d.Node)
	}

	var edges []*graph.Edge
	for _, e := range g.Edges() {
		if _, ok := found[e.SrcID]; !ok {
			continue
		}
		if _, ok := found[e.DstID]; !ok {
			continue
		}
		edges = append(edges, e)
	}
	return nodes, edges
}
