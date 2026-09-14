package goparser

import (
	"testing"

	"kgraph/internal/graph"
)

func extractFixture(t *testing.T) *graph.Graph {
	t.Helper()
	ext := NewGoExtractor()
	g, warnings, err := ext.ExtractRepo("testdata/fixture")
	if err != nil {
		t.Fatalf("ExtractRepo() error = %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}
	return g
}

func mustNode(t *testing.T, g *graph.Graph, id string) *graph.Node {
	t.Helper()
	n := g.Node(id)
	if n == nil {
		t.Fatalf("expected node %q to exist", id)
	}
	return n
}

func hasEdge(g *graph.Graph, edgeType graph.EdgeType, srcID, dstID string) bool {
	for _, e := range g.OutEdges(srcID) {
		if e.Type == edgeType && e.DstID == dstID {
			return true
		}
	}
	return false
}

func TestExtractStructsAndFields(t *testing.T) {
	g := extractFixture(t)

	userID := graph.StructID("fixture", "User")
	mustNode(t, g, userID)

	for _, field := range []string{"ID", "Name", "Email"} {
		fid := graph.FieldID(userID, field)
		mustNode(t, g, fid)
		if !hasEdge(g, graph.EdgeTypeHasField, userID, fid) {
			t.Errorf("expected has_field edge %s -> %s", userID, fid)
		}
	}
}

func TestExtractMethodWithReceiver(t *testing.T) {
	g := extractFixture(t)

	orderID := graph.StructID("fixture", "Order")
	methodID := graph.FunctionID("fixture", "Order", "Summary")
	mustNode(t, g, methodID)

	if !hasEdge(g, graph.EdgeTypeHasMethod, orderID, methodID) {
		t.Errorf("expected has_method edge %s -> %s", orderID, methodID)
	}
}

func TestExtractCallEdges(t *testing.T) {
	g := extractFixture(t)

	summaryID := graph.FunctionID("fixture", "Order", "Summary")
	greetingID := graph.FunctionID("fixture", "", "GreetingFor")
	itoaID := graph.FunctionID("fixture", "", "itoa")

	if !hasEdge(g, graph.EdgeTypeCalls, summaryID, greetingID) {
		t.Errorf("expected calls edge %s -> %s", summaryID, greetingID)
	}
	if !hasEdge(g, graph.EdgeTypeCalls, summaryID, itoaID) {
		t.Errorf("expected calls edge %s -> %s", summaryID, itoaID)
	}
}

func TestExtractORMTagsToTableAndColumns(t *testing.T) {
	g := extractFixture(t)

	usersTable := mustNode(t, g, graph.TableID("users"))
	if usersTable.Type != graph.NodeTypeTable {
		t.Errorf("expected users table node to have Type Table, got %s", usersTable.Type)
	}

	userStructID := graph.StructID("fixture", "User")
	if !hasEdge(g, graph.EdgeTypeMapsToTable, userStructID, graph.TableID("users")) {
		t.Errorf("expected maps_to_table edge %s -> table:users", userStructID)
	}

	for _, col := range []string{"id", "name", "email"} {
		mustNode(t, g, graph.ColumnID("users", col))
	}

	fkSrc := graph.ColumnID("orders", "user_id")
	if !hasEdge(g, graph.EdgeTypeReferencesFK, fkSrc, graph.TableID("users")) {
		t.Errorf("expected references_fk edge %s -> table:users", fkSrc)
	}
}

func TestExtractMigrationReconciliation(t *testing.T) {
	g := extractFixture(t)

	usersTable := mustNode(t, g, graph.TableID("users"))
	source, _ := usersTable.Properties["source"].(string)
	if source != "migration+struct_tag" {
		t.Errorf("expected users table source to be migration+struct_tag after reconciliation, got %q", source)
	}

	idCol := mustNode(t, g, graph.ColumnID("users", "id"))
	if sqlType, _ := idCol.Properties["sql_type"].(string); sqlType != "INT" {
		t.Errorf("expected migration to set sql_type=INT on users.id, got %q", sqlType)
	}
	if _, hadConflict := idCol.Properties["struct_tag_conflict"]; !hadConflict {
		t.Errorf("expected users.id to record its prior struct_tag properties as struct_tag_conflict")
	}

	if !hasEdge(g, graph.EdgeTypeReferencesFK, graph.TableID("orders"), graph.TableID("users")) {
		t.Errorf("expected references_fk edge table:orders -> table:users from the migration's FOREIGN KEY constraint")
	}
}

func TestExtractRepoTagsEveryNodeWithLanguage(t *testing.T) {
	g := extractFixture(t)

	if g.NodeCount() == 0 {
		t.Fatal("fixture produced no nodes")
	}

	byType := map[graph.NodeType]int{}
	for _, n := range g.Nodes() {
		lang, _ := n.Properties["language"].(string)
		if lang != "go" {
			t.Errorf("node %s (type %s) has Properties[\"language\"] = %q, want \"go\"", n.ID, n.Type, lang)
		}
		byType[n.Type]++
	}

	for _, nt := range []graph.NodeType{
		graph.NodeTypeExternalDependency,
		graph.NodeTypeTable,
		graph.NodeTypeColumn,
	} {
		if byType[nt] == 0 {
			t.Errorf("fixture produced no %s nodes; this test cannot verify language tagging for that node type", nt)
		}
	}
}

func TestExtractRepoIdempotent(t *testing.T) {
	g1 := extractFixture(t)
	g2 := extractFixture(t)

	if g1.NodeCount() != g2.NodeCount() {
		t.Fatalf("node count differs across runs: %d vs %d", g1.NodeCount(), g2.NodeCount())
	}
	for _, n1 := range g1.Nodes() {
		n2 := g2.Node(n1.ID)
		if n2 == nil {
			t.Fatalf("node %s present in first run but missing in second", n1.ID)
		}
		if n1.Hash != n2.Hash {
			t.Errorf("hash for node %s changed across runs with no source changes: %s vs %s", n1.ID, n1.Hash, n2.Hash)
		}
	}
}
