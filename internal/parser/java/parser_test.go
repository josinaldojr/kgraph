package javaparser

import (
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
)

func extractFixture(t *testing.T) *graph.Graph {
	t.Helper()
	ext := NewJavaExtractor()
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

func TestExtractPackages(t *testing.T) {
	g := extractFixture(t)

	for _, pkg := range []string{
		"com.example.demo.model",
		"com.example.demo.repository",
		"com.example.demo.service",
		"com.example.demo.controller",
	} {
		n := mustNode(t, g, graph.PackageID(pkg))
		if n.Type != graph.NodeTypePackage {
			t.Errorf("package %s: got Type %s, want Package", pkg, n.Type)
		}
	}
}

func TestExtractClassesAndInterfaces(t *testing.T) {
	g := extractFixture(t)

	userID := graph.StructID("com.example.demo.model", "User")
	mustNode(t, g, userID)

	repoID := graph.InterfaceID("com.example.demo.repository", "UserRepository")
	n := mustNode(t, g, repoID)
	if n.Type != graph.NodeTypeInterface {
		t.Errorf("UserRepository: got Type %s, want Interface", n.Type)
	}
}

func TestJPAEntityMapping(t *testing.T) {
	g := extractFixture(t)

	userID := graph.StructID("com.example.demo.model", "User")
	tableID := graph.TableID("users")
	mustNode(t, g, tableID)
	if !hasEdge(g, graph.EdgeTypeMapsToTable, userID, tableID) {
		t.Errorf("expected %s --maps_to_table--> %s", userID, tableID)
	}

	idColID := graph.ColumnID("users", "id")
	idCol := mustNode(t, g, idColID)
	if idCol.Type != graph.NodeTypeColumn {
		t.Errorf("id column: got Type %s, want Column", idCol.Type)
	}
	if !hasEdge(g, graph.EdgeTypeHasField, tableID, idColID) {
		t.Errorf("expected %s --has_field--> %s", tableID, idColID)
	}

	nameColID := graph.ColumnID("users", "user_name")
	mustNode(t, g, nameColID)
	if !hasEdge(g, graph.EdgeTypeHasField, tableID, nameColID) {
		t.Errorf("expected %s --has_field--> %s", tableID, nameColID)
	}

	idFieldID := graph.FieldID(userID, "id")
	idField := mustNode(t, g, idFieldID)
	if generated, _ := idField.Properties["generated"].(bool); !generated {
		t.Errorf("id field: expected generated=true, got %v", idField.Properties["generated"])
	}
}

func TestBareTableAnnotationWithoutEntity(t *testing.T) {
	g := extractFixture(t)

	noteID := graph.StructID("com.example.demo.model", "Note")
	mustNode(t, g, noteID)

	if g.Node(graph.TableID("notes")) != nil {
		t.Errorf("did not expect a Table node for a class annotated @Table without @Entity")
	}
	if hasEdge(g, graph.EdgeTypeMapsToTable, noteID, graph.TableID("notes")) {
		t.Errorf("did not expect %s --maps_to_table--> %s without @Entity", noteID, graph.TableID("notes"))
	}
}

func TestSpringAnnotations(t *testing.T) {
	g := extractFixture(t)

	serviceID := graph.StructID("com.example.demo.service", "UserService")
	serviceDecID := graph.DecoratorID("com.example.demo.service", "Service")
	mustNode(t, g, serviceDecID)
	if !hasEdge(g, graph.EdgeTypeDecorated, serviceID, serviceDecID) {
		t.Errorf("expected %s --decorated--> %s", serviceID, serviceDecID)
	}

	controllerID := graph.StructID("com.example.demo.controller", "UserController")
	restDecID := graph.DecoratorID("com.example.demo.controller", "RestController")
	mustNode(t, g, restDecID)
	if !hasEdge(g, graph.EdgeTypeDecorated, controllerID, restDecID) {
		t.Errorf("expected %s --decorated--> %s", controllerID, restDecID)
	}
}

func TestSpringDependencyInjection(t *testing.T) {
	g := extractFixture(t)

	serviceID := graph.StructID("com.example.demo.service", "UserService")
	repoID := graph.InterfaceID("com.example.demo.repository", "UserRepository")
	if !hasEdge(g, graph.EdgeTypeInjected, serviceID, repoID) {
		t.Errorf("expected %s --injected--> %s", serviceID, repoID)
	}

	controllerID := graph.StructID("com.example.demo.controller", "UserController")
	if !hasEdge(g, graph.EdgeTypeInjected, controllerID, serviceID) {
		t.Errorf("expected %s --injected--> %s", controllerID, serviceID)
	}
}

func TestSpringRouting(t *testing.T) {
	g := extractFixture(t)

	funcID := graph.FunctionID("com.example.demo.controller", "", "getUser")
	mustNode(t, g, funcID)

	epID := graph.EndpointID("GET", "/api/users/{id}")
	ep := mustNode(t, g, epID)
	if ep.Type != graph.NodeTypeEndpoint {
		t.Errorf("endpoint: got Type %s, want Endpoint", ep.Type)
	}
	if !hasEdge(g, graph.EdgeTypeRouted, funcID, epID) {
		t.Errorf("expected %s --routed--> %s", funcID, epID)
	}
}

func TestExternalImportEdgeToExternalDependency(t *testing.T) {
	g := extractFixture(t)

	controllerPkgID := graph.PackageID("com.example.demo.controller")
	depID := graph.ExternalDependencyID("org.springframework")
	if !hasEdge(g, graph.EdgeTypeImports, controllerPkgID, depID) {
		t.Errorf("expected %s --imports--> %s", controllerPkgID, depID)
	}
}

func TestInternalImportEdgeToPackage(t *testing.T) {
	g := extractFixture(t)

	controllerPkgID := graph.PackageID("com.example.demo.controller")
	modelPkgID := graph.PackageID("com.example.demo.model")
	if !hasEdge(g, graph.EdgeTypeImports, controllerPkgID, modelPkgID) {
		t.Errorf("expected %s --imports--> %s", controllerPkgID, modelPkgID)
	}
}

func TestExtractPackagesScoped(t *testing.T) {
	ext := NewJavaExtractor()

	g, _, err := ext.ExtractPackages("testdata/fixture", nil, nil)
	if err != nil {
		t.Fatalf("ExtractPackages(nil patterns) error = %v", err)
	}
	if g.NodeCount() != 0 {
		t.Errorf("empty patterns: got %d nodes, want 0", g.NodeCount())
	}

	g, _, err = ext.ExtractPackages("testdata/fixture", []string{"./src/main/java/com/example/demo/model"}, nil)
	if err != nil {
		t.Fatalf("ExtractPackages(...) error = %v", err)
	}
	mustNode(t, g, graph.StructID("com.example.demo.model", "User"))
	if g.Node(graph.StructID("com.example.demo.service", "UserService")) != nil {
		t.Errorf("scoped ExtractPackages should not have extracted UserService")
	}
}

func TestSpringRoutingMultiLineAnnotation(t *testing.T) {
	g := extractFixture(t)

	funcID := graph.FunctionID("com.example.demo.controller", "", "deleteUser")
	mustNode(t, g, funcID)

	epID := graph.EndpointID("DELETE", "/api/users/{id}")
	mustNode(t, g, epID)
	if !hasEdge(g, graph.EdgeTypeRouted, funcID, epID) {
		t.Errorf("expected %s --routed--> %s", funcID, epID)
	}
}

func TestMethodCalls(t *testing.T) {
	g := extractFixture(t)

	loadUserID := graph.FunctionID("com.example.demo.service", "", "loadUser")
	getUserID := graph.FunctionID("com.example.demo.service", "", "getUser")
	mustNode(t, g, loadUserID)
	mustNode(t, g, getUserID)

	if !hasEdge(g, graph.EdgeTypeCalls, loadUserID, getUserID) {
		t.Errorf("expected %s --calls--> %s", loadUserID, getUserID)
	}
}

func TestMavenDependencies(t *testing.T) {
	g := extractFixture(t)

	depID := graph.ExternalDependencyID("org.springframework.boot:spring-boot-starter-web")
	n := mustNode(t, g, depID)
	if n.Type != graph.NodeTypeExternalDependency {
		t.Errorf("dependency: got Type %s, want ExternalDependency", n.Type)
	}
}
