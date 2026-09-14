package graph

import "testing"

func TestEdgeConfidenceRoundTripsThroughOutAndInEdges(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddNode(&Node{ID: "c"})

	if err := g.AddEdge(&Edge{ID: "extracted", Type: EdgeTypeCalls, SrcID: "a", DstID: "b", Confidence: ConfidenceExtracted}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	if err := g.AddEdge(&Edge{ID: "inferred", Type: EdgeTypeImplements, SrcID: "a", DstID: "c", Confidence: ConfidenceInferred}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	out := g.OutEdges("a")
	if len(out) != 2 {
		t.Fatalf("OutEdges(a) = %d edges, want 2", len(out))
	}
	byID := map[string]*Edge{out[0].ID: out[0], out[1].ID: out[1]}
	if byID["extracted"].Confidence != ConfidenceExtracted {
		t.Errorf("extracted edge Confidence = %q, want %q", byID["extracted"].Confidence, ConfidenceExtracted)
	}
	if byID["inferred"].Confidence != ConfidenceInferred {
		t.Errorf("inferred edge Confidence = %q, want %q", byID["inferred"].Confidence, ConfidenceInferred)
	}

	inB := g.InEdges("b")
	if len(inB) != 1 || inB[0].Confidence != ConfidenceExtracted {
		t.Errorf("InEdges(b) = %+v, want one edge with Confidence EXTRACTED", inB)
	}
	inC := g.InEdges("c")
	if len(inC) != 1 || inC[0].Confidence != ConfidenceInferred {
		t.Errorf("InEdges(c) = %+v, want one edge with Confidence INFERRED", inC)
	}
}

func TestEdgeIDIsIndependentOfConfidence(t *testing.T) {
	// design.md Decision 1.8: confidence doesn't affect edge identity, so
	// the same logical edge with a different confidence value still
	// produces the same deterministic ID.
	a := EdgeID(EdgeTypeCalls, "x", "y")
	b := EdgeID(EdgeTypeCalls, "x", "y")
	if a != b {
		t.Errorf("EdgeID() should not depend on Confidence: got %q and %q", a, b)
	}
}
