package pythonparser

import (
	"strings"
	"testing"

	"kgraph/internal/graph"
)

func extractFixture(t *testing.T) *graph.Graph {
	t.Helper()
	ext := NewPythonExtractor()
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

func TestExtractClasses(t *testing.T) {
	g := extractFixture(t)

	userID := graph.StructID("myapp.models", "User")
	mustNode(t, g, userID)

	dtoID := graph.StructID("myapp.models", "UserDTO")
	dto := mustNode(t, g, dtoID)
	if dataclass, _ := dto.Properties["dataclass"].(bool); !dataclass {
		t.Errorf("UserDTO: expected dataclass=true, got %v", dto.Properties["dataclass"])
	}
}

func TestDataclassDecorator(t *testing.T) {
	g := extractFixture(t)

	dtoID := graph.StructID("myapp.models", "UserDTO")
	decID := graph.DecoratorID("myapp.models", "dataclass")
	mustNode(t, g, decID)
	if !hasEdge(g, graph.EdgeTypeDecorated, dtoID, decID) {
		t.Errorf("expected %s --decorated--> %s", dtoID, decID)
	}
}

func TestSQLAlchemyModelExtraction(t *testing.T) {
	g := extractFixture(t)

	userID := graph.StructID("myapp.models", "User")
	tableID := graph.TableID("py_users")
	mustNode(t, g, tableID)
	if !hasEdge(g, graph.EdgeTypeMapsToTable, userID, tableID) {
		t.Errorf("expected %s --maps_to_table--> %s", userID, tableID)
	}

	idColID := graph.ColumnID("py_users", "id")
	idCol := mustNode(t, g, idColID)
	if idCol.Type != graph.NodeTypeColumn {
		t.Errorf("id column: got Type %s, want Column", idCol.Type)
	}
	if !hasEdge(g, graph.EdgeTypeHasField, tableID, idColID) {
		t.Errorf("expected %s --has_field--> %s", tableID, idColID)
	}

	idFieldID := graph.FieldID(userID, "id")
	idField := mustNode(t, g, idFieldID)
	if typ, _ := idField.Properties["type"].(string); typ != "Integer" {
		t.Errorf("id field: got type %v, want Integer", idField.Properties["type"])
	}

	nameColID := graph.ColumnID("py_users", "name")
	mustNode(t, g, nameColID)
	if !hasEdge(g, graph.EdgeTypeHasField, tableID, nameColID) {
		t.Errorf("expected %s --has_field--> %s", tableID, nameColID)
	}
}

func TestFlaskRouting(t *testing.T) {
	g := extractFixture(t)

	funcID := graph.FunctionID("myapp.app", "", "get_users")
	mustNode(t, g, funcID)

	epID := graph.EndpointID("GET", "/users")
	ep := mustNode(t, g, epID)
	if ep.Type != graph.NodeTypeEndpoint {
		t.Errorf("endpoint: got Type %s, want Endpoint", ep.Type)
	}
	if !hasEdge(g, graph.EdgeTypeRouted, funcID, epID) {
		t.Errorf("expected %s --routed--> %s", funcID, epID)
	}
}

func TestFastAPIRouting(t *testing.T) {
	g := extractFixture(t)

	getFuncID := graph.FunctionID("myapp.app", "", "get_user")
	getEpID := graph.EndpointID("GET", "/users/{user_id}")
	mustNode(t, g, getEpID)
	if !hasEdge(g, graph.EdgeTypeRouted, getFuncID, getEpID) {
		t.Errorf("expected %s --routed--> %s", getFuncID, getEpID)
	}

	createFuncID := graph.FunctionID("myapp.app", "", "create_user")
	createEpID := graph.EndpointID("POST", "/users")
	mustNode(t, g, createEpID)
	if !hasEdge(g, graph.EdgeTypeRouted, createFuncID, createEpID) {
		t.Errorf("expected %s --routed--> %s", createFuncID, createEpID)
	}
}

func TestFlaskRoutingMultiLineDecorator(t *testing.T) {
	g := extractFixture(t)

	funcID := graph.FunctionID("myapp.app", "", "delete_user")
	mustNode(t, g, funcID)

	epID := graph.EndpointID("DELETE", "/users/<int:user_id>")
	mustNode(t, g, epID)
	if !hasEdge(g, graph.EdgeTypeRouted, funcID, epID) {
		t.Errorf("expected %s --routed--> %s", funcID, epID)
	}
}

func TestMethodCalls(t *testing.T) {
	g := extractFixture(t)

	getUserID := graph.FunctionID("myapp.service", "", "get_user")
	loadUserID := graph.FunctionID("myapp.service", "", "load_user")
	mustNode(t, g, getUserID)
	mustNode(t, g, loadUserID)

	if !hasEdge(g, graph.EdgeTypeCalls, getUserID, loadUserID) {
		t.Errorf("expected %s --calls--> %s", getUserID, loadUserID)
	}
}

func TestExternalImportEdgeToExternalDependency(t *testing.T) {
	g := extractFixture(t)

	appPkgID := graph.PackageID("myapp.app")
	depID := graph.ExternalDependencyID("flask")
	if !hasEdge(g, graph.EdgeTypeImports, appPkgID, depID) {
		t.Errorf("expected %s --imports--> %s", appPkgID, depID)
	}

	stubID := graph.PackageID("flask")
	if g.Node(stubID) != nil {
		t.Errorf("did not expect a bare Package node %s for an external import", stubID)
	}
}

func TestRelativeImportEdgeToInternalPackage(t *testing.T) {
	g := extractFixture(t)

	appPkgID := graph.PackageID("myapp.app")
	servicePkgID := graph.PackageID("myapp.service")
	if !hasEdge(g, graph.EdgeTypeImports, appPkgID, servicePkgID) {
		t.Errorf("expected %s --imports--> %s (from relative import)", appPkgID, servicePkgID)
	}

	servicePkgID2 := graph.PackageID("myapp.models")
	serviceSrcID := graph.PackageID("myapp.service")
	if !hasEdge(g, graph.EdgeTypeImports, serviceSrcID, servicePkgID2) {
		t.Errorf("expected %s --imports--> %s (from relative import)", serviceSrcID, servicePkgID2)
	}
}

func TestExtractPackagesScoped(t *testing.T) {
	ext := NewPythonExtractor()

	g, _, err := ext.ExtractPackages("testdata/fixture", nil, nil)
	if err != nil {
		t.Fatalf("ExtractPackages(nil patterns) error = %v", err)
	}
	if g.NodeCount() != 0 {
		t.Errorf("empty patterns: got %d nodes, want 0", g.NodeCount())
	}

	g, _, err = ext.ExtractPackages("testdata/fixture", []string{"./myapp"}, nil)
	if err != nil {
		t.Fatalf("ExtractPackages([./myapp]) error = %v", err)
	}
	mustNode(t, g, graph.PackageID("myapp.app"))
	mustNode(t, g, graph.PackageID("myapp.service"))
}

func TestABCProducesInterfaceNotStruct(t *testing.T) {
	g := extractFixture(t)

	repoID := graph.StructID("myapp.interfaces", "Repository")
	n := mustNode(t, g, repoID)
	if n.Type != graph.NodeTypeInterface {
		t.Errorf("Repository: got Type %s, want Interface", n.Type)
	}

	for _, conflict := range g.IDConflicts() {
		if strings.Contains(conflict, repoID) {
			t.Errorf("unexpected ID conflict for %s: %s", repoID, conflict)
		}
	}
}

func TestRequirementsDependencies(t *testing.T) {
	g := extractFixture(t)

	depID := graph.ExternalDependencyID("flask")
	n := mustNode(t, g, depID)
	if n.Type != graph.NodeTypeExternalDependency {
		t.Errorf("dependency: got Type %s, want ExternalDependency", n.Type)
	}
	if version, _ := n.Properties["version"].(string); version != ">=2.0.0" {
		t.Errorf("flask dependency: got version %q, want %q", version, ">=2.0.0")
	}
}
