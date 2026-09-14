package common_test

import (
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/parser/common"
	javaparser "github.com/josinaldojr/kgraph/internal/parser/java"
	pythonparser "github.com/josinaldojr/kgraph/internal/parser/python"
	typescriptparser "github.com/josinaldojr/kgraph/internal/parser/typescript"
)

// TestMergeGraphsMultiLanguage exercises MergeGraphsWithWarnings against
// the real Java, TypeScript and Python extractor fixtures together, as a
// stand-in for a genuine multi-language repository (task 7.5 / 8.6).
func TestMergeGraphsMultiLanguage(t *testing.T) {
	javaGraph, _, err := javaparser.NewJavaExtractor().ExtractRepo("../java/testdata/fixture")
	if err != nil {
		t.Fatalf("java ExtractRepo() error = %v", err)
	}
	tsGraph, _, err := typescriptparser.NewTypeScriptExtractor().ExtractRepo("../typescript/testdata/fixture")
	if err != nil {
		t.Fatalf("typescript ExtractRepo() error = %v", err)
	}
	pyGraph, _, err := pythonparser.NewPythonExtractor().ExtractRepo("../python/testdata/fixture")
	if err != nil {
		t.Fatalf("python ExtractRepo() error = %v", err)
	}

	wantNodes := javaGraph.NodeCount() + tsGraph.NodeCount() + pyGraph.NodeCount()
	wantEdges := javaGraph.EdgeCount() + tsGraph.EdgeCount() + pyGraph.EdgeCount()

	merged, warnings := common.MergeGraphsWithWarnings(javaGraph, tsGraph, pyGraph)

	// Each fixture happens to declare a class named "User", but the
	// language-specific package/module prefix keeps their IDs distinct
	// (Decision 5: namespace collisions across languages are rare in
	// practice because ecosystem naming conventions already diverge) —
	// so merging three independently-extracted graphs should conflict on
	// nothing and lose nothing.
	if len(warnings) != 0 {
		t.Errorf("expected no merge warnings, got %v", warnings)
	}
	if merged.NodeCount() != wantNodes {
		t.Errorf("merged NodeCount() = %d, want %d", merged.NodeCount(), wantNodes)
	}
	if merged.EdgeCount() != wantEdges {
		t.Errorf("merged EdgeCount() = %d, want %d", merged.EdgeCount(), wantEdges)
	}

	for _, id := range []string{
		graph.StructID("com.example.demo.model", "User"),
		graph.StructID("src.user.entity", "User"),
		graph.StructID("myapp.models", "User"),
	} {
		if merged.Node(id) == nil {
			t.Errorf("expected merged graph to contain node %q", id)
		}
	}
}

// TestMergeGraphsWithWarningsDetectsConflict verifies that merging two
// graphs which assign the same ID to nodes of different Type is reported
// as a warning, with last-write-wins semantics (see graph.AddNode).
func TestMergeGraphsWithWarningsDetectsConflict(t *testing.T) {
	const collidingID = "pkg.Foo"

	a := graph.New()
	a.AddNode(&graph.Node{ID: collidingID, Type: graph.NodeTypeStruct})

	b := graph.New()
	b.AddNode(&graph.Node{ID: collidingID, Type: graph.NodeTypeVariable})

	merged, warnings := common.MergeGraphsWithWarnings(a, b)

	if len(warnings) == 0 {
		t.Fatal("expected a warning for the colliding node ID, got none")
	}

	got := merged.Node(collidingID)
	if got == nil {
		t.Fatal("expected merged graph to contain the colliding node")
	}
	if got.Type != graph.NodeTypeVariable {
		t.Errorf("collidingID: got Type %s, want %s (last-write-wins)", got.Type, graph.NodeTypeVariable)
	}
}

// TestKnownInternalForLanguage verifies that Package nodes are filtered by
// their "language" property, so an incremental update for one language
// doesn't get handed package names that belong to another.
func TestKnownInternalForLanguage(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{
		ID:         graph.PackageID("com.example.service"),
		Type:       graph.NodeTypePackage,
		Properties: map[string]any{"language": string(common.LangJava)},
	})
	g.AddNode(&graph.Node{
		ID:         graph.PackageID("mypackage.users"),
		Type:       graph.NodeTypePackage,
		Properties: map[string]any{"language": string(common.LangPython)},
	})

	javaInternal := common.KnownInternalForLanguage(g, common.LangJava)
	if !javaInternal["com.example.service"] {
		t.Errorf("expected com.example.service to be known internal for java, got %v", javaInternal)
	}
	if javaInternal["mypackage.users"] {
		t.Errorf("expected mypackage.users to NOT be known internal for java, got %v", javaInternal)
	}

	pyInternal := common.KnownInternalForLanguage(g, common.LangPython)
	if !pyInternal["mypackage.users"] {
		t.Errorf("expected mypackage.users to be known internal for python, got %v", pyInternal)
	}
	if pyInternal["com.example.service"] {
		t.Errorf("expected com.example.service to NOT be known internal for python, got %v", pyInternal)
	}
}
