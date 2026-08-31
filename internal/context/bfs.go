package context

import "kgraph/internal/graph"

// discovered tracks one node found during subgraph expansion: how far
// (hops) from the target it is, the strongest edge type that reached it
// (for priority ranking), and the parent/edge that discovered it (for a
// compact per-node relation line in the rendered output).
type discovered struct {
	Node        *graph.Node
	Hop         int
	Weight      int
	ParentID    string
	ViaEdge     graph.EdgeType
	ViaOutgoing bool // true if the edge went parent -> node (node is "downstream")
}

// edgeWeight prioritizes structurally meaningful edges (calls, has_method)
// over merely referential ones (imports) when ranking/truncating, per
// design.md Decision 8.
func edgeWeight(t graph.EdgeType) int {
	switch t {
	case graph.EdgeTypeCalls, graph.EdgeTypeHasMethod:
		return 3
	case graph.EdgeTypeHasField, graph.EdgeTypeMapsToTable, graph.EdgeTypeReferencesFK,
		graph.EdgeTypeReadsTable, graph.EdgeTypeWritesTable, graph.EdgeTypeExposesEndpoint,
		graph.EdgeTypeEmbeds, graph.EdgeTypeImplements:
		return 2
	default: // imports
		return 1
	}
}

// expandSubgraph performs a breadth-first expansion from target up to
// hops edges (both directions — a target's callers matter as much as its
// callees), returning every discovered node (target included, Hop 0)
// keyed by ID.
func expandSubgraph(g *graph.Graph, target *graph.Node, hops int) map[string]*discovered {
	found := map[string]*discovered{
		target.ID: {Node: target, Hop: 0},
	}
	frontier := []string{target.ID}

	for level := 1; level <= hops; level++ {
		var next []string
		for _, id := range frontier {
			for _, e := range g.OutEdges(id) {
				considerNeighbor(g, found, &next, id, e.DstID, e.Type, true, level)
			}
			for _, e := range g.InEdges(id) {
				considerNeighbor(g, found, &next, id, e.SrcID, e.Type, false, level)
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	return found
}

func considerNeighbor(g *graph.Graph, found map[string]*discovered, next *[]string, parentID, neighborID string, edgeType graph.EdgeType, outgoing bool, level int) {
	if neighborID == parentID {
		return
	}
	w := edgeWeight(edgeType)
	if existing, ok := found[neighborID]; ok {
		if w > existing.Weight {
			existing.Weight = w
			existing.ViaEdge = edgeType
			existing.ParentID = parentID
			existing.ViaOutgoing = outgoing
		}
		return
	}
	n := g.Node(neighborID)
	if n == nil {
		return
	}
	found[neighborID] = &discovered{
		Node: n, Hop: level, Weight: w,
		ParentID: parentID, ViaEdge: edgeType, ViaOutgoing: outgoing,
	}
	*next = append(*next, neighborID)
}
