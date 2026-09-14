package context

import (
	"container/heap"
	"fmt"

	"kgraph/internal/graph"
)

// PathHop is one node along a Path result: the node itself, and — except
// for the first hop (the source) — the edge type and confidence of the
// edge that reached it from the previous hop.
type PathHop struct {
	Node       *graph.Node
	ViaEdge    graph.EdgeType
	Confidence string
}

// edgeCost inverts edgeWeight so Dijkstra (which minimizes cost) prefers
// structurally meaningful edges (calls, has_method) over merely
// referential ones (imports) — design.md Decision 6.
func edgeCost(t graph.EdgeType) int {
	return 4 - edgeWeight(t)
}

type pathQueueItem struct {
	id   string
	cost int
}

type pathQueue []pathQueueItem

func (q pathQueue) Len() int            { return len(q) }
func (q pathQueue) Less(i, j int) bool  { return q[i].cost < q[j].cost }
func (q pathQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *pathQueue) Push(x interface{}) { *q = append(*q, x.(pathQueueItem)) }
func (q *pathQueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[:n-1]
	return item
}

// Path finds the lowest-cost path between srcID and dstID using Dijkstra
// over the graph treated as undirected (a target's callers matter as much
// as its callees, matching expandSubgraph), where cost prefers
// structurally meaningful edges over merely referential ones. Returns an
// error if either node doesn't exist or no path connects them.
func Path(g *graph.Graph, srcID, dstID string) ([]PathHop, error) {
	if g.Node(srcID) == nil {
		return nil, fmt.Errorf("context: unknown source node %q", srcID)
	}
	if g.Node(dstID) == nil {
		return nil, fmt.Errorf("context: unknown destination node %q", dstID)
	}
	if srcID == dstID {
		return []PathHop{{Node: g.Node(srcID)}}, nil
	}

	dist := map[string]int{srcID: 0}
	prevEdge := map[string]*graph.Edge{}
	prevNode := map[string]string{}
	visited := map[string]bool{}

	pq := &pathQueue{{id: srcID, cost: 0}}
	heap.Init(pq)

	for pq.Len() > 0 {
		cur := heap.Pop(pq).(pathQueueItem)
		if visited[cur.id] {
			continue
		}
		visited[cur.id] = true
		if cur.id == dstID {
			break
		}

		neighbors := append(append([]*graph.Edge{}, g.OutEdges(cur.id)...), g.InEdges(cur.id)...)
		for _, e := range neighbors {
			neighborID := e.DstID
			if neighborID == cur.id {
				neighborID = e.SrcID
			}
			if neighborID == cur.id || visited[neighborID] {
				continue
			}
			nd := dist[cur.id] + edgeCost(e.Type)
			if existing, ok := dist[neighborID]; !ok || nd < existing {
				dist[neighborID] = nd
				prevEdge[neighborID] = e
				prevNode[neighborID] = cur.id
				heap.Push(pq, pathQueueItem{id: neighborID, cost: nd})
			}
		}
	}

	if _, ok := dist[dstID]; !ok {
		return nil, fmt.Errorf("context: no path from %q to %q", srcID, dstID)
	}

	var hops []PathHop
	id := dstID
	for id != srcID {
		e := prevEdge[id]
		hops = append([]PathHop{{Node: g.Node(id), ViaEdge: e.Type, Confidence: e.Confidence}}, hops...)
		id = prevNode[id]
	}
	hops = append([]PathHop{{Node: g.Node(srcID)}}, hops...)
	return hops, nil
}
