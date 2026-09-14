package typescriptparser

import (
	"testing"

	"kgraph/internal/graph"
)

func extractFixture(t *testing.T) *graph.Graph {
	t.Helper()
	ext := NewTypeScriptExtractor()
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

func TestExtractClassesAndInterfaces(t *testing.T) {
	g := extractFixture(t)

	userID := graph.StructID("src.user.entity", "User")
	mustNode(t, g, userID)

	repoID := graph.InterfaceID("src.user.repository", "UserRepository")
	n := mustNode(t, g, repoID)
	if n.Type != graph.NodeTypeInterface {
		t.Errorf("UserRepository: got Type %s, want Interface", n.Type)
	}
}

func TestTypeORMEntityMapping(t *testing.T) {
	g := extractFixture(t)

	userID := graph.StructID("src.user.entity", "User")
	tableID := graph.TableID("ts_users")
	mustNode(t, g, tableID)
	if !hasEdge(g, graph.EdgeTypeMapsToTable, userID, tableID) {
		t.Errorf("expected %s --maps_to_table--> %s", userID, tableID)
	}

	idColID := graph.ColumnID("ts_users", "id")
	idCol := mustNode(t, g, idColID)
	if idCol.Type != graph.NodeTypeColumn {
		t.Errorf("id column: got Type %s, want Column", idCol.Type)
	}
	if !hasEdge(g, graph.EdgeTypeHasField, tableID, idColID) {
		t.Errorf("expected %s --has_field--> %s", tableID, idColID)
	}

	nameColID := graph.ColumnID("ts_users", "user_name")
	mustNode(t, g, nameColID)
	if !hasEdge(g, graph.EdgeTypeHasField, tableID, nameColID) {
		t.Errorf("expected %s --has_field--> %s", tableID, nameColID)
	}
}

func TestNestJSDecorators(t *testing.T) {
	g := extractFixture(t)

	serviceID := graph.StructID("src.user.service", "UserService")
	injectableDecID := graph.DecoratorID("src.user.service", "Injectable")
	mustNode(t, g, injectableDecID)
	if !hasEdge(g, graph.EdgeTypeDecorated, serviceID, injectableDecID) {
		t.Errorf("expected %s --decorated--> %s", serviceID, injectableDecID)
	}

	controllerID := graph.StructID("src.user.controller", "UserController")
	controllerDecID := graph.DecoratorID("src.user.controller", "Controller")
	mustNode(t, g, controllerDecID)
	if !hasEdge(g, graph.EdgeTypeDecorated, controllerID, controllerDecID) {
		t.Errorf("expected %s --decorated--> %s", controllerID, controllerDecID)
	}
}

func TestNestJSDependencyInjection(t *testing.T) {
	g := extractFixture(t)

	serviceID := graph.StructID("src.user.service", "UserService")
	repoID := graph.InterfaceID("src.user.repository", "UserRepository")
	if !hasEdge(g, graph.EdgeTypeInjected, serviceID, repoID) {
		t.Errorf("expected %s --injected--> %s", serviceID, repoID)
	}

	controllerID := graph.StructID("src.user.controller", "UserController")
	if !hasEdge(g, graph.EdgeTypeInjected, controllerID, serviceID) {
		t.Errorf("expected %s --injected--> %s", controllerID, serviceID)
	}
}

func TestNestJSRouting(t *testing.T) {
	g := extractFixture(t)

	funcID := graph.FunctionID("src.user.controller", "", "getUser")
	mustNode(t, g, funcID)

	epID := graph.EndpointID("GET", "users/:id")
	ep := mustNode(t, g, epID)
	if ep.Type != graph.NodeTypeEndpoint {
		t.Errorf("endpoint: got Type %s, want Endpoint", ep.Type)
	}
	if !hasEdge(g, graph.EdgeTypeRouted, funcID, epID) {
		t.Errorf("expected %s --routed--> %s", funcID, epID)
	}

	createFuncID := graph.FunctionID("src.user.controller", "", "createUser")
	createEpID := graph.EndpointID("POST", "users")
	mustNode(t, g, createEpID)
	if !hasEdge(g, graph.EdgeTypeRouted, createFuncID, createEpID) {
		t.Errorf("expected %s --routed--> %s", createFuncID, createEpID)
	}
}

func TestNestJSRoutingMultiLineDecorator(t *testing.T) {
	g := extractFixture(t)

	funcID := graph.FunctionID("src.user.controller", "", "deleteUser")
	mustNode(t, g, funcID)

	epID := graph.EndpointID("DELETE", "users/:id")
	mustNode(t, g, epID)
	if !hasEdge(g, graph.EdgeTypeRouted, funcID, epID) {
		t.Errorf("expected %s --routed--> %s", funcID, epID)
	}
}

func TestExternalImportEdgeToExternalDependency(t *testing.T) {
	g := extractFixture(t)

	servicePkgID := graph.PackageID("src.user.service")
	depID := graph.ExternalDependencyID("@nestjs/core")
	if !hasEdge(g, graph.EdgeTypeImports, servicePkgID, depID) {
		t.Errorf("expected %s --imports--> %s", servicePkgID, depID)
	}
}

func TestRelativeImportEdgeToInternalPackage(t *testing.T) {
	g := extractFixture(t)

	servicePkgID := graph.PackageID("src.user.service")
	repoPkgID := graph.PackageID("src.user.repository")
	if !hasEdge(g, graph.EdgeTypeImports, servicePkgID, repoPkgID) {
		t.Errorf("expected %s --imports--> %s", servicePkgID, repoPkgID)
	}
}

func TestExtractPackagesScoped(t *testing.T) {
	ext := NewTypeScriptExtractor()

	g, _, err := ext.ExtractPackages("testdata/fixture", nil, nil)
	if err != nil {
		t.Fatalf("ExtractPackages(nil patterns) error = %v", err)
	}
	if g.NodeCount() != 0 {
		t.Errorf("empty patterns: got %d nodes, want 0", g.NodeCount())
	}

	g, _, err = ext.ExtractPackages("testdata/fixture", []string{"./src"}, nil)
	if err != nil {
		t.Fatalf("ExtractPackages([./src]) error = %v", err)
	}
	mustNode(t, g, graph.StructID("src.user.service", "UserService"))
}

func TestMethodCalls(t *testing.T) {
	g := extractFixture(t)

	loadUserID := graph.FunctionID("src.user.service", "", "loadUser")
	getUserID := graph.FunctionID("src.user.service", "", "getUser")
	mustNode(t, g, loadUserID)
	mustNode(t, g, getUserID)

	if !hasEdge(g, graph.EdgeTypeCalls, loadUserID, getUserID) {
		t.Errorf("expected %s --calls--> %s", loadUserID, getUserID)
	}
}

func TestPackageJSONDependencies(t *testing.T) {
	g := extractFixture(t)

	depID := graph.ExternalDependencyID("@nestjs/core")
	n := mustNode(t, g, depID)
	if n.Type != graph.NodeTypeExternalDependency {
		t.Errorf("dependency: got Type %s, want ExternalDependency", n.Type)
	}
	if lang, _ := n.Properties["language"].(string); lang != "typescript" {
		t.Errorf("dependency: got Properties[\"language\"] = %q, want \"typescript\"", lang)
	}
}

func TestPackageJSONDependenciesJavaScriptMode(t *testing.T) {
	ext := NewJavaScriptExtractor()
	g, warnings, err := ext.ExtractRepo("testdata/fixture")
	if err != nil {
		t.Fatalf("ExtractRepo() error = %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}

	depID := graph.ExternalDependencyID("@nestjs/core")
	n := mustNode(t, g, depID)
	if lang, _ := n.Properties["language"].(string); lang != "javascript" {
		t.Errorf("dependency under JavaScript-mode extraction: got Properties[\"language\"] = %q, want \"javascript\"", lang)
	}

	devDepID := graph.ExternalDependencyID("typescript")
	devDep := mustNode(t, g, devDepID)
	if lang, _ := devDep.Properties["language"].(string); lang != "javascript" {
		t.Errorf("devDependency under JavaScript-mode extraction: got Properties[\"language\"] = %q, want \"javascript\"", lang)
	}
}
