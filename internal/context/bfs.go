package context

import "github.com/josinaldojr/kgraph/internal/graph"

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
	ViaOutgoing bool   // true if the edge went parent -> node (node is "downstream")
	Confidence  string // Confidence of the edge that connects this node to its parent
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

// communityBoost is added to an edge's base weight when the neighbor it
// reaches shares the target's community, per context-assembly's
// community-aware relevance requirement: nodes in the same subsystem as
// the target should rank ahead of equally-connected nodes elsewhere.
const communityBoost = 1

// expandSubgraph performs a breadth-first expansion from target up to
// hops edges (both directions — a target's callers matter as much as its
// callees), returning every discovered node (target included, Hop 0)
// keyed by ID. Nodes sharing the target's community (if analytics has run)
// get a relevance boost so they rank ahead of equally-connected nodes in
// other communities.
func expandSubgraph(g *graph.Graph, target *graph.Node, hops int) map[string]*discovered {
	found := map[string]*discovered{
		target.ID: {Node: target, Hop: 0},
	}
	frontier := []string{target.ID}
	targetCommunity := target.Community()

	for level := 1; level <= hops; level++ {
		var next []string
		for _, id := range frontier {
			for _, e := range g.OutEdges(id) {
				considerNeighbor(g, found, &next, id, e.DstID, e, true, level, targetCommunity)
			}
			for _, e := range g.InEdges(id) {
				considerNeighbor(g, found, &next, id, e.SrcID, e, false, level, targetCommunity)
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	return found
}

func considerNeighbor(g *graph.Graph, found map[string]*discovered, next *[]string, parentID, neighborID string, edge *graph.Edge, outgoing bool, level, targetCommunity int) {
	if neighborID == parentID {
		return
	}
	n := g.Node(neighborID)
	if n == nil {
		return
	}
	w := edgeWeight(edge.Type)
	if targetCommunity != -1 && n.Community() == targetCommunity {
		w += communityBoost
	}
	if existing, ok := found[neighborID]; ok {
		if w > existing.Weight {
			existing.Weight = w
			existing.ViaEdge = edge.Type
			existing.ParentID = parentID
			existing.ViaOutgoing = outgoing
			existing.Confidence = edge.Confidence
		}
		return
	}
	found[neighborID] = &discovered{
		Node: n, Hop: level, Weight: w,
		ParentID: parentID, ViaEdge: edge.Type, ViaOutgoing: outgoing, Confidence: edge.Confidence,
	}
	*next = append(*next, neighborID)
}
