package analytics

import (
	"sort"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// DefaultGodNodeCount is the default N used by `kgraph build`/`kgraph
// analyze` when --god-nodes isn't given, per graph-analytics' "default
// N=10" requirement.
const DefaultGodNodeCount = 10

// DetectGodNodes marks the top n nodes by degree (Node.Degree, so
// ComputeDegree must have run first) with Properties["god_node"] = true,
// and every other node with Properties["god_node"] = false, per
// graph-analytics' "God nodes SHALL be detected and marked" requirement.
// Ties at the n-th rank are broken by node ID for determinism.
func DetectGodNodes(g *graph.Graph, n int) {
	nodes := g.Nodes()
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Degree() != nodes[j].Degree() {
			return nodes[i].Degree() > nodes[j].Degree()
		}
		return nodes[i].ID < nodes[j].ID
	})

	for i, node := range nodes {
		if node.Properties == nil {
			node.Properties = make(map[string]any)
		}
		node.Properties[graph.PropertyGodNode] = i < n
	}
}
