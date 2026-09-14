// Package graph implements the in-memory code knowledge graph: plain maps
// for nodes and adjacency, with no external graph library.
package graph

import (
	"fmt"
	"sort"
)

// Graph is an in-memory, typed graph of code entities and relationships.
type Graph struct {
	nodes map[string]*Node
	edges map[string]*Edge
	// out maps a node ID to the IDs of edges where it is the source.
	out map[string][]string
	// in maps a node ID to the IDs of edges where it is the destination.
	in map[string][]string
	// idConflicts records AddNode calls that overwrote a node of a
	// different Type under the same ID — see AddNode.
	idConflicts []string
}

// New returns an empty Graph.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
		edges: make(map[string]*Edge),
		out:   make(map[string][]string),
		in:    make(map[string][]string),
	}
}

// AddNode inserts or replaces a node by ID. Re-adding a node with the same
// ID and the same Type is normal — rebuilds and incremental updates do this
// on every run, since IDs are deterministic (see ids.go) — so that alone is
// not recorded as a conflict. But an existing node being replaced by one of
// a *different* Type is almost certainly two unrelated entities colliding
// on ID (e.g. a Struct and a Variable sharing importPath+name), not a
// re-save; see IDConflicts.
func (g *Graph) AddNode(n *Node) {
	if n == nil {
		return
	}
	if existing, ok := g.nodes[n.ID]; ok && existing.Type != n.Type {
		g.idConflicts = append(g.idConflicts, fmt.Sprintf(
			"graph: ID conflict: %s already exists as %s, overwritten by %s",
			n.ID, existing.Type, n.Type,
		))
	}
	g.nodes[n.ID] = n
}

// IDConflicts returns warnings recorded by AddNode for node ID collisions
// between nodes of different types. Callers (extractors, build/update)
// should fold these into their own warning output.
func (g *Graph) IDConflicts() []string {
	return g.idConflicts
}

// AddEdge inserts or replaces an edge by ID and indexes it for adjacency
// lookups. Both endpoints must already exist as nodes.
//
// Re-adding the same edge ID with the same SrcID/DstID (the normal case:
// EdgeID is derived from exactly those two, so a rebuild re-adding an
// unchanged edge always does this) is a no-op for the index — out/in
// already point at it. If SrcID/DstID ever differ from what's currently
// stored under that ID, the stale index entries are removed and fresh ones
// added, so out/in never point a caller at an edge whose current endpoints
// don't match. No extractor or merge path in this codebase does that
// today; this is defensive hardening of the invariant, not a fix for an
// observed defect.
func (g *Graph) AddEdge(e *Edge) error {
	if e == nil {
		return fmt.Errorf("graph: nil edge")
	}
	if _, ok := g.nodes[e.SrcID]; !ok {
		return fmt.Errorf("graph: edge %s references unknown src node %s", e.ID, e.SrcID)
	}
	if _, ok := g.nodes[e.DstID]; !ok {
		return fmt.Errorf("graph: edge %s references unknown dst node %s", e.ID, e.DstID)
	}

	if existing, exists := g.edges[e.ID]; !exists {
		g.out[e.SrcID] = append(g.out[e.SrcID], e.ID)
		g.in[e.DstID] = append(g.in[e.DstID], e.ID)
	} else if existing.SrcID != e.SrcID || existing.DstID != e.DstID {
		g.out[existing.SrcID] = removeString(g.out[existing.SrcID], e.ID)
		g.in[existing.DstID] = removeString(g.in[existing.DstID], e.ID)
		g.out[e.SrcID] = append(g.out[e.SrcID], e.ID)
		g.in[e.DstID] = append(g.in[e.DstID], e.ID)
	}

	g.edges[e.ID] = e
	return nil
}

// removeString returns ids with the first occurrence of id removed.
func removeString(ids []string, id string) []string {
	for i, existing := range ids {
		if existing == id {
			return append(ids[:i:i], ids[i+1:]...)
		}
	}
	return ids
}

// Node returns the node with the given ID, or nil if it doesn't exist.
func (g *Graph) Node(id string) *Node {
	return g.nodes[id]
}

// Nodes returns every node in the graph. Order is not guaranteed.
func (g *Graph) Nodes() []*Node {
	out := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	return out
}

// Edges returns every edge in the graph. Order is not guaranteed.
func (g *Graph) Edges() []*Edge {
	out := make([]*Edge, 0, len(g.edges))
	for _, e := range g.edges {
		out = append(out, e)
	}
	return out
}

// OutEdges returns the edges where the given node ID is the source.
func (g *Graph) OutEdges(id string) []*Edge {
	ids := g.out[id]
	out := make([]*Edge, 0, len(ids))
	for _, eid := range ids {
		if e, ok := g.edges[eid]; ok {
			out = append(out, e)
		}
	}
	return out
}

// InEdges returns the edges where the given node ID is the destination.
func (g *Graph) InEdges(id string) []*Edge {
	ids := g.in[id]
	out := make([]*Edge, 0, len(ids))
	for _, eid := range ids {
		if e, ok := g.edges[eid]; ok {
			out = append(out, e)
		}
	}
	return out
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int { return len(g.edges) }

// GodNodes returns up to n nodes flagged as god nodes (Node.IsGodNode),
// sorted by descending degree. Requires the analyze stage to have run.
func (g *Graph) GodNodes(n int) []*Node {
	var candidates []*Node
	for _, node := range g.nodes {
		if node.IsGodNode() {
			candidates = append(candidates, node)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Degree() > candidates[j].Degree()
	})
	if n >= 0 && n < len(candidates) {
		candidates = candidates[:n]
	}
	return candidates
}

// NodesByCommunity returns every node assigned to the given community ID.
// Requires the analyze stage to have run.
func (g *Graph) NodesByCommunity(id int) []*Node {
	var out []*Node
	for _, node := range g.nodes {
		if node.Community() == id {
			out = append(out, node)
		}
	}
	return out
}

// Communities groups every node with an assigned community ID by that ID.
// Nodes without a community (Community() == -1) are omitted.
func (g *Graph) Communities() map[int][]*Node {
	out := make(map[int][]*Node)
	for _, node := range g.nodes {
		id := node.Community()
		if id == -1 {
			continue
		}
		out[id] = append(out[id], node)
	}
	return out
}
