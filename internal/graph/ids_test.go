package graph

import "testing"

// idCase is a table-driven determinism check: calling fn with the same
// inputs twice must produce the same ID, and each entry's ID must differ
// from every other entry's (different logical inputs never collide).
type idCase struct {
	name string
	id   string
}

func assertDeterministicAndDistinct(t *testing.T, cases []idCase) {
	t.Helper()
	seen := make(map[string]string, len(cases))
	for _, c := range cases {
		if prevName, ok := seen[c.id]; ok {
			t.Errorf("%s and %s produced the same ID %q, want distinct IDs", prevName, c.name, c.id)
		}
		seen[c.id] = c.name
	}
}

func TestPackageIDDeterministic(t *testing.T) {
	if a, b := PackageID("kgraph/internal/graph"), PackageID("kgraph/internal/graph"); a != b {
		t.Errorf("PackageID() not deterministic: %q != %q", a, b)
	}
	assertDeterministicAndDistinct(t, []idCase{
		{"graph", PackageID("kgraph/internal/graph")},
		{"store", PackageID("kgraph/internal/store")},
	})
}

func TestStructIDDeterministic(t *testing.T) {
	if a, b := StructID("fixture", "User"), StructID("fixture", "User"); a != b {
		t.Errorf("StructID() not deterministic: %q != %q", a, b)
	}
	assertDeterministicAndDistinct(t, []idCase{
		{"fixture.User", StructID("fixture", "User")},
		{"fixture.Order", StructID("fixture", "Order")},
		{"other.User", StructID("other", "User")},
	})
}

func TestFunctionIDPlainVsMethod(t *testing.T) {
	plain := FunctionID("fixture", "", "GreetingFor")
	if plain != "fixture.GreetingFor()" {
		t.Errorf("FunctionID(plain) = %q, want %q", plain, "fixture.GreetingFor()")
	}
	method := FunctionID("fixture", "Order", "Summary")
	if method != "fixture.Order.Summary()" {
		t.Errorf("FunctionID(method) = %q, want %q", method, "fixture.Order.Summary()")
	}
	assertDeterministicAndDistinct(t, []idCase{
		{"plain GreetingFor", plain},
		{"Order.Summary method", method},
		{"Invoice.Summary method (different receiver)", FunctionID("fixture", "Invoice", "Summary")},
		{"plain Summary (no receiver)", FunctionID("fixture", "", "Summary")},
	})
}

func TestFieldIDDeterministic(t *testing.T) {
	owner := StructID("fixture", "User")
	assertDeterministicAndDistinct(t, []idCase{
		{"User.ID", FieldID(owner, "ID")},
		{"User.Name", FieldID(owner, "Name")},
	})
	if a, b := FieldID(owner, "ID"), FieldID(owner, "ID"); a != b {
		t.Errorf("FieldID() not deterministic: %q != %q", a, b)
	}
}

func TestTableAndColumnIDDeterministic(t *testing.T) {
	assertDeterministicAndDistinct(t, []idCase{
		{"users table", TableID("users")},
		{"orders table", TableID("orders")},
		{"users.id column", ColumnID("users", "id")},
		{"users.name column", ColumnID("users", "name")},
		{"orders.id column", ColumnID("orders", "id")},
	})
}

func TestEndpointIDDeterministic(t *testing.T) {
	assertDeterministicAndDistinct(t, []idCase{
		{"GET /users", EndpointID("GET", "/users")},
		{"POST /users", EndpointID("POST", "/users")},
		{"GET /orders", EndpointID("GET", "/orders")},
	})
}

func TestExternalDependencyIDDeterministic(t *testing.T) {
	assertDeterministicAndDistinct(t, []idCase{
		{"gorm", ExternalDependencyID("gorm.io/gorm")},
		{"cobra", ExternalDependencyID("github.com/spf13/cobra")},
	})
}

func TestEdgeIDDeterministic(t *testing.T) {
	if a, b := EdgeID(EdgeTypeCalls, "a", "b"), EdgeID(EdgeTypeCalls, "a", "b"); a != b {
		t.Errorf("EdgeID() not deterministic: %q != %q", a, b)
	}
	assertDeterministicAndDistinct(t, []idCase{
		{"calls a->b", EdgeID(EdgeTypeCalls, "a", "b")},
		{"calls b->a (reversed)", EdgeID(EdgeTypeCalls, "b", "a")},
		{"imports a->b (different type)", EdgeID(EdgeTypeImports, "a", "b")},
	})
}

// TestCrossTypeIDsDoNotCollide guards the bug found in the 2026-09-13
// audit: Enum/Variable/TypeAlias were added using the same bare
// "importPath.name" scheme as Struct/Interface, so e.g. a Struct and a
// Variable with the same name in the same package silently produced the
// same ID. Struct vs. Interface intentionally stays out of this check —
// per design.md Decision 7, Go/Java/TS/Python all forbid a class and an
// interface sharing a name in the same scope, so that pair was never the
// collision risk; Enum/Variable/TypeAlias don't have that guarantee
// (e.g. Python freely lets a module-level variable and a function share
// a name across statements), which is why only those three were prefixed.
func TestCrossTypeIDsDoNotCollide(t *testing.T) {
	assertDeterministicAndDistinct(t, []idCase{
		{"struct", StructID("pkg", "Foo")},
		{"enum", EnumID("pkg", "Foo")},
		{"variable", VariableID("pkg", "Foo")},
		{"typealias", TypeAliasID("pkg", "Foo")},
	})
}

func TestEnumDecoratorVariableTypeAliasIDsDeterministic(t *testing.T) {
	assertDeterministicAndDistinct(t, []idCase{
		{"enum Status", EnumID("fixture", "Status")},
		{"enum Role", EnumID("fixture", "Role")},
		{"decorator Retry", DecoratorID("fixture", "Retry")},
		{"decorator Cache", DecoratorID("fixture", "Cache")},
		{"variable DefaultTimeout", VariableID("fixture", "DefaultTimeout")},
		{"typealias UserID", TypeAliasID("fixture", "UserID")},
	})
}

func TestRationaleIDDeterministic(t *testing.T) {
	if a, b := RationaleID("f.go", "10", "WHY"), RationaleID("f.go", "10", "WHY"); a != b {
		t.Errorf("RationaleID() not deterministic: %q != %q", a, b)
	}
	assertDeterministicAndDistinct(t, []idCase{
		{"f.go:10 WHY", RationaleID("f.go", "10", "WHY")},
		{"f.go:11 WHY (different line)", RationaleID("f.go", "11", "WHY")},
		{"f.go:10 NOTE (different kind)", RationaleID("f.go", "10", "NOTE")},
		{"g.go:10 WHY (different file)", RationaleID("g.go", "10", "WHY")},
	})
}
