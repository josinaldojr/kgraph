// Package graph implements the in-memory code knowledge graph: plain maps
// for nodes and adjacency, with no external graph library.
package graph

import "fmt"

// Graph is an in-memory, typed graph of code entities and relationships.
type Graph struct {
	nodes map[string]*Node
	edges map[string]*Edge
	// out maps a node ID to the IDs of edges where it is the source.
	out map[string][]string
	// in maps a node ID to the IDs of edges where it is the destination.
	in map[string][]string
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

// AddNode inserts or replaces a node by ID.
func (g *Graph) AddNode(n *Node) {
	if n == nil {
		return
	}
	g.nodes[n.ID] = n
}

// AddEdge inserts or replaces an edge by ID and indexes it for adjacency
// lookups. Both endpoints must already exist as nodes.
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
	if _, exists := g.edges[e.ID]; !exists {
		g.out[e.SrcID] = append(g.out[e.SrcID], e.ID)
		g.in[e.DstID] = append(g.in[e.DstID], e.ID)
	}
	g.edges[e.ID] = e
	return nil
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
