// Package analytics computes structural properties of an already-parsed,
// enriched graph — degree, god nodes, and communities — as the Analyze
// stage that runs after Parse and Enrich in the build pipeline, per
// design.md Decision 3. Results are stored as keys in Node.Properties
// (design.md Decision 1) rather than typed fields, so no schema migration
// is needed.
package analytics

import "kgraph/internal/graph"

// ComputeDegree sets Properties["degree"] on every node in g to its total
// degree (incoming + outgoing edge count), per graph-analytics' "Every
// node SHALL have a computed degree" requirement.
func ComputeDegree(g *graph.Graph) {
	for _, n := range g.Nodes() {
		degree := len(g.OutEdges(n.ID)) + len(g.InEdges(n.ID))
		if n.Properties == nil {
			n.Properties = make(map[string]any)
		}
		n.Properties[graph.PropertyDegree] = degree
	}
}
