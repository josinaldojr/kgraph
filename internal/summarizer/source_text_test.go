package summarizer

import (
	"strings"
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// buildTableFixtureGraph builds two ORM/migration-style tables: `users`
// (two columns) and `orders`, whose `user_id` column references `users` via
// a references_fk edge — so `users` has an incoming FK to render.
func buildTableFixtureGraph() *graph.Graph {
	g := graph.New()

	g.AddNode(&graph.Node{
		ID: graph.TableID("users"), Type: graph.NodeTypeTable,
		Hash: "table-users-1", Properties: map[string]any{"name": "users", "source": "migration"},
	})
	g.AddNode(&graph.Node{
		ID: graph.ColumnID("users", "id"), Type: graph.NodeTypeColumn,
		Hash: "col-users-id-1", Properties: map[string]any{"name": "id", "sql_type": "INT"},
	})
	g.AddNode(&graph.Node{
		ID: graph.ColumnID("users", "email"), Type: graph.NodeTypeColumn,
		Hash: "col-users-email-1", Properties: map[string]any{"name": "email", "sql_type": "VARCHAR(255)"},
	})
	_ = g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeHasField, SrcID: graph.TableID("users"), DstID: graph.ColumnID("users", "id")})
	_ = g.AddEdge(&graph.Edge{ID: "e2", Type: graph.EdgeTypeHasField, SrcID: graph.TableID("users"), DstID: graph.ColumnID("users", "email")})

	g.AddNode(&graph.Node{
		ID: graph.TableID("orders"), Type: graph.NodeTypeTable,
		Hash: "table-orders-1", Properties: map[string]any{"name": "orders", "source": "migration"},
	})
	g.AddNode(&graph.Node{
		ID: graph.ColumnID("orders", "user_id"), Type: graph.NodeTypeColumn,
		Hash: "col-orders-user-id-1", Properties: map[string]any{"name": "user_id", "sql_type": "INT"},
	})
	_ = g.AddEdge(&graph.Edge{ID: "e3", Type: graph.EdgeTypeHasField, SrcID: graph.TableID("orders"), DstID: graph.ColumnID("orders", "user_id")})
	_ = g.AddEdge(&graph.Edge{ID: "e4", Type: graph.EdgeTypeReferencesFK, SrcID: graph.ColumnID("orders", "user_id"), DstID: graph.TableID("users")})

	return g
}

func buildEndpointFixtureGraph() *graph.Graph {
	g := graph.New()
	g.AddNode(&graph.Node{
		ID: "demo.HandleUsers()", Type: graph.NodeTypeFunction,
		Signature: "func HandleUsers(w http.ResponseWriter, r *http.Request)",
		Hash:      "handler-hash-1",
	})
	epID := graph.EndpointID("GET", "/users")
	g.AddNode(&graph.Node{ID: epID, Type: graph.NodeTypeEndpoint, Hash: "endpoint-hash-1"})
	_ = g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeExposesEndpoint, SrcID: "demo.HandleUsers()", DstID: epID})
	return g
}

func buildExternalDepFixtureGraph() *graph.Graph {
	g := graph.New()
	g.AddNode(&graph.Node{
		ID: graph.ExternalDependencyID("github.com/spf13/cobra"), Type: graph.NodeTypeExternalDependency,
		Hash: "ext-1", Properties: map[string]any{"import_path": "github.com/spf13/cobra"},
	})
	g.AddNode(&graph.Node{
		ID: graph.ExternalDependencyID("golang.org/x/mod"), Type: graph.NodeTypeExternalDependency,
		Hash: "ext-2", Properties: map[string]any{"import_path": "golang.org/x/mod", "version": "v0.39.0"},
	})
	return g
}

func TestPendingNodesIncludesTableSourceText(t *testing.T) {
	g := buildTableFixtureGraph()
	s := openTestStore(t)

	pending, err := PendingSummaries(g, s, LevelNode, 0)
	if err != nil {
		t.Fatalf("PendingSummaries() error = %v", err)
	}

	byID := make(map[string]PendingSummary, len(pending))
	for _, p := range pending {
		byID[p.ID] = p
	}

	users, ok := byID[graph.TableID("users")]
	if !ok {
		t.Fatalf("expected table:users to be pending, got %+v", pending)
	}
	for _, want := range []string{"id INT", "email VARCHAR(255)", "Referenced by (foreign key): " + graph.ColumnID("orders", "user_id")} {
		if !strings.Contains(users.Source, want) {
			t.Errorf("expected users source text to contain %q, got:\n%s", want, users.Source)
		}
	}

	if _, ok := byID[graph.ColumnID("users", "id")]; ok {
		t.Error("expected Column node to be excluded from pending export, per proposal.md's scope decision")
	}
}

func TestPendingNodesExcludesFieldAndColumn(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "demo.Widget", Type: graph.NodeTypeStruct, Hash: "struct-1", File: "x.go", LineStart: 1, LineEnd: 3})
	g.AddNode(&graph.Node{ID: graph.FieldID("demo.Widget", "Name"), Type: graph.NodeTypeField, Hash: "field-1"})
	g.AddNode(&graph.Node{ID: graph.TableID("widgets"), Type: graph.NodeTypeTable, Hash: "table-1"})
	g.AddNode(&graph.Node{ID: graph.ColumnID("widgets", "name"), Type: graph.NodeTypeColumn, Hash: "col-1"})

	pending, err := PendingSummaries(g, openTestStore(t), LevelNode, 0)
	if err != nil {
		t.Fatalf("PendingSummaries() error = %v", err)
	}
	for _, p := range pending {
		if p.Type == string(graph.NodeTypeField) || p.Type == string(graph.NodeTypeColumn) {
			t.Errorf("expected Field/Column never to appear in pending export, got %+v", p)
		}
	}
}

func TestPendingNodesIncludesEndpointSourceText(t *testing.T) {
	g := buildEndpointFixtureGraph()
	pending, err := PendingSummaries(g, openTestStore(t), LevelNode, 0)
	if err != nil {
		t.Fatalf("PendingSummaries() error = %v", err)
	}

	var ep *PendingSummary
	for i := range pending {
		if pending[i].ID == graph.EndpointID("GET", "/users") {
			ep = &pending[i]
		}
	}
	if ep == nil {
		t.Fatalf("expected the endpoint to be pending, got %+v", pending)
	}
	for _, want := range []string{"GET /users", "Handler: func HandleUsers(w http.ResponseWriter, r *http.Request)"} {
		if !strings.Contains(ep.Source, want) {
			t.Errorf("expected endpoint source text to contain %q, got:\n%s", want, ep.Source)
		}
	}
}

func TestPendingNodesIncludesExternalDependencySourceText(t *testing.T) {
	g := buildExternalDepFixtureGraph()
	pending, err := PendingSummaries(g, openTestStore(t), LevelNode, 0)
	if err != nil {
		t.Fatalf("PendingSummaries() error = %v", err)
	}

	byID := make(map[string]PendingSummary, len(pending))
	for _, p := range pending {
		byID[p.ID] = p
	}

	noVersion, ok := byID[graph.ExternalDependencyID("github.com/spf13/cobra")]
	if !ok || noVersion.Source != "EXTERNAL DEPENDENCY github.com/spf13/cobra\n" {
		t.Errorf("unexpected source for version-less dependency: %+v", noVersion)
	}
	withVersion, ok := byID[graph.ExternalDependencyID("golang.org/x/mod")]
	if !ok || withVersion.Source != "EXTERNAL DEPENDENCY golang.org/x/mod @ v0.39.0\n" {
		t.Errorf("unexpected source for versioned dependency: %+v", withVersion)
	}
}

func TestApplySummariesForNewTypes(t *testing.T) {
	g := buildTableFixtureGraph()
	s := openTestStore(t)

	res, err := ApplySummaries(g, s, LevelNode, "test-model", []AppliedSummary{
		{ID: graph.TableID("users"), Hash: "table-users-1", Summary: "Stores registered user accounts."},
	})
	if err != nil {
		t.Fatalf("ApplySummaries() error = %v", err)
	}
	if len(res.Applied) != 1 || res.Applied[0] != graph.TableID("users") {
		t.Fatalf("expected table:users applied, got %+v", res)
	}

	rec, ok, err := s.GetSummary(graph.TableID("users"), LevelNode)
	if err != nil || !ok || rec.Summary != "Stores registered user accounts." {
		t.Fatalf("expected stored summary for table:users, got %+v ok=%v err=%v", rec, ok, err)
	}
}
