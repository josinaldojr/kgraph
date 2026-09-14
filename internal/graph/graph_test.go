package graph

import "testing"

func nodeWith(id string, community int, degree int, godNode bool) *Node {
	return &Node{
		ID:   id,
		Type: NodeTypeFunction,
		Properties: map[string]any{
			PropertyCommunity: community,
			PropertyDegree:    degree,
			PropertyGodNode:   godNode,
		},
	}
}

func analyzedFixture() *Graph {
	g := New()
	g.AddNode(nodeWith("a", 0, 10, true))
	g.AddNode(nodeWith("b", 0, 3, false))
	g.AddNode(nodeWith("c", 1, 7, true))
	g.AddNode(nodeWith("d", 1, 1, false))
	g.AddNode(&Node{ID: "e", Type: NodeTypeFunction}) // no analytics run
	return g
}

func TestGraphGodNodes(t *testing.T) {
	g := analyzedFixture()

	all := g.GodNodes(-1)
	if len(all) != 2 {
		t.Fatalf("GodNodes(-1) = %d nodes, want 2", len(all))
	}
	if all[0].ID != "a" || all[1].ID != "c" {
		t.Errorf("GodNodes(-1) not ordered by descending degree: got %s, %s", all[0].ID, all[1].ID)
	}

	top1 := g.GodNodes(1)
	if len(top1) != 1 || top1[0].ID != "a" {
		t.Errorf("GodNodes(1) = %+v, want just node a", top1)
	}

	// n larger than the candidate count: no truncation, no panic.
	all3 := g.GodNodes(10)
	if len(all3) != 2 {
		t.Errorf("GodNodes(10) = %d nodes, want 2 (truncation only applies when n < len)", len(all3))
	}
}

func TestGraphNodesByCommunity(t *testing.T) {
	g := analyzedFixture()

	community0 := g.NodesByCommunity(0)
	if len(community0) != 2 {
		t.Fatalf("NodesByCommunity(0) = %d nodes, want 2", len(community0))
	}
	ids := map[string]bool{}
	for _, n := range community0 {
		ids[n.ID] = true
	}
	if !ids["a"] || !ids["b"] {
		t.Errorf("NodesByCommunity(0) = %+v, want a and b", community0)
	}

	if got := g.NodesByCommunity(99); len(got) != 0 {
		t.Errorf("NodesByCommunity(99) = %+v, want empty", got)
	}
}

func TestGraphCommunities(t *testing.T) {
	g := analyzedFixture()

	communities := g.Communities()
	if len(communities) != 2 {
		t.Fatalf("Communities() = %d groups, want 2", len(communities))
	}
	if len(communities[0]) != 2 || len(communities[1]) != 2 {
		t.Errorf("Communities() = %+v, want 2 members in each of communities 0 and 1", communities)
	}

	// Node "e" never had analytics run (Community() == -1) and must be
	// excluded from every group.
	for id, members := range communities {
		for _, n := range members {
			if n.ID == "e" {
				t.Errorf("community %d must not include node e (no assigned community), got %+v", id, members)
			}
		}
	}
}

func TestGraphOutInEdges(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	if err := g.AddEdge(&Edge{ID: "e1", Type: EdgeTypeCalls, SrcID: "a", DstID: "b"}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	out := g.OutEdges("a")
	if len(out) != 1 || out[0].ID != "e1" {
		t.Errorf("OutEdges(a) = %+v, want [e1]", out)
	}
	in := g.InEdges("b")
	if len(in) != 1 || in[0].ID != "e1" {
		t.Errorf("InEdges(b) = %+v, want [e1]", in)
	}
	if len(g.OutEdges("b")) != 0 || len(g.InEdges("a")) != 0 {
		t.Error("expected no reverse-direction adjacency")
	}
}

func TestGraphAddEdgeRejectsUnknownEndpoints(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})

	if err := g.AddEdge(&Edge{ID: "e1", SrcID: "a", DstID: "missing"}); err == nil {
		t.Error("expected an error for an edge referencing an unknown destination node")
	}
	if err := g.AddEdge(&Edge{ID: "e2", SrcID: "missing", DstID: "a"}); err == nil {
		t.Error("expected an error for an edge referencing an unknown source node")
	}
	if err := g.AddEdge(nil); err == nil {
		t.Error("expected an error for a nil edge")
	}
}

func TestGraphAddEdgeReindexesOnEndpointChange(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddNode(&Node{ID: "c"})

	if err := g.AddEdge(&Edge{ID: "e1", Type: EdgeTypeCalls, SrcID: "a", DstID: "b"}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	// Re-add the same edge ID with different endpoints (not a pattern any
	// extractor uses today, since EdgeID is derived from SrcID/DstID — but
	// AddEdge's out/in indices must not go stale if it ever happens).
	if err := g.AddEdge(&Edge{ID: "e1", Type: EdgeTypeCalls, SrcID: "a", DstID: "c"}); err != nil {
		t.Fatalf("AddEdge() (endpoint change) error = %v", err)
	}

	if out := g.OutEdges("a"); len(out) != 1 || out[0].DstID != "c" {
		t.Errorf("OutEdges(a) = %+v, want exactly one edge to c", out)
	}
	if in := g.InEdges("b"); len(in) != 0 {
		t.Errorf("InEdges(b) = %+v, want none — the edge no longer targets b", in)
	}
	if in := g.InEdges("c"); len(in) != 1 || in[0].ID != "e1" {
		t.Errorf("InEdges(c) = %+v, want [e1]", in)
	}
}

func TestGraphAddNodeIDConflicts(t *testing.T) {
	g := New()

	// Re-adding the same ID with the same Type is a normal re-save (rebuilds
	// and incremental updates do this every run) and must not be flagged.
	g.AddNode(&Node{ID: "pkg.Foo", Type: NodeTypeStruct, Hash: "v1"})
	g.AddNode(&Node{ID: "pkg.Foo", Type: NodeTypeStruct, Hash: "v2"})
	if got := g.IDConflicts(); len(got) != 0 {
		t.Fatalf("IDConflicts() = %v, want none for same-Type re-save", got)
	}

	// Re-adding the same ID with a different Type is the collision this
	// method exists to catch (e.g. StructID/VariableID sharing a scheme).
	g.AddNode(&Node{ID: "pkg.Foo", Type: NodeTypeVariable})
	conflicts := g.IDConflicts()
	if len(conflicts) != 1 {
		t.Fatalf("IDConflicts() = %v, want exactly 1 conflict", conflicts)
	}
	if g.Node("pkg.Foo").Type != NodeTypeVariable {
		t.Error("AddNode should still overwrite on a type conflict (best-effort), not reject it")
	}
}

func TestGraphNodeAndEdgeCounts(t *testing.T) {
	g := New()
	if g.NodeCount() != 0 || g.EdgeCount() != 0 {
		t.Fatal("expected a fresh graph to be empty")
	}
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	if err := g.AddEdge(&Edge{ID: "e1", SrcID: "a", DstID: "b"}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	if g.NodeCount() != 2 {
		t.Errorf("NodeCount() = %d, want 2", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() = %d, want 1", g.EdgeCount())
	}

	// Re-adding an edge with the same ID must not double-count adjacency.
	if err := g.AddEdge(&Edge{ID: "e1", SrcID: "a", DstID: "b"}); err != nil {
		t.Fatalf("AddEdge() (re-add) error = %v", err)
	}
	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() after re-adding the same edge = %d, want 1", g.EdgeCount())
	}
	if len(g.OutEdges("a")) != 1 {
		t.Errorf("OutEdges(a) after re-adding the same edge = %d, want 1", len(g.OutEdges("a")))
	}
}
