package common

import "testing"

func TestScanBalanced_Simple(t *testing.T) {
	s := "('/users', methods=['GET'])"
	content, end, ok := ScanBalanced(s, 0, '(', ')')
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := "'/users', methods=['GET']"
	if content != want {
		t.Errorf("content: got %q, want %q", content, want)
	}
	if end != len(s) {
		t.Errorf("end: got %d, want %d", end, len(s))
	}
}

func TestScanBalanced_Nested(t *testing.T) {
	s := "(foo(bar(baz)), qux)"
	content, end, ok := ScanBalanced(s, 0, '(', ')')
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := "foo(bar(baz)), qux"
	if content != want {
		t.Errorf("content: got %q, want %q", content, want)
	}
	if end != len(s) {
		t.Errorf("end: got %d, want %d", end, len(s))
	}
}

func TestScanBalanced_Brackets(t *testing.T) {
	s := "['GET', 'POST']"
	content, _, ok := ScanBalanced(s, 0, '[', ']')
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if content != "'GET', 'POST'" {
		t.Errorf("content: got %q", content)
	}
}

func TestScanBalanced_Unbalanced(t *testing.T) {
	s := "(foo(bar"
	_, _, ok := ScanBalanced(s, 0, '(', ')')
	if ok {
		t.Fatalf("expected ok=false for unbalanced input")
	}
}

func TestScanBalanced_NotAtOpenDelimiter(t *testing.T) {
	s := "foo(bar)"
	_, _, ok := ScanBalanced(s, 0, '(', ')')
	if ok {
		t.Fatalf("expected ok=false when start does not index the open delimiter")
	}
}

func TestScanBalanced_QuotedDelimitersIgnored(t *testing.T) {
	s := `("has ) inside", 'and ( too')`
	content, end, ok := ScanBalanced(s, 0, '(', ')')
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := `"has ) inside", 'and ( too'`
	if content != want {
		t.Errorf("content: got %q, want %q", content, want)
	}
	if end != len(s) {
		t.Errorf("end: got %d, want %d", end, len(s))
	}
}

func TestScanBalanced_EscapedQuoteInsideString(t *testing.T) {
	s := `("a \" b )")`
	content, _, ok := ScanBalanced(s, 0, '(', ')')
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := `"a \" b )"`
	if content != want {
		t.Errorf("content: got %q, want %q", content, want)
	}
}

func TestScanBalanced_MultiLine(t *testing.T) {
	s := "(\n  '/users',\n  methods=['POST'],\n)"
	content, end, ok := ScanBalanced(s, 0, '(', ')')
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := "\n  '/users',\n  methods=['POST'],\n"
	if content != want {
		t.Errorf("content: got %q, want %q", content, want)
	}
	if end != len(s) {
		t.Errorf("end: got %d, want %d", end, len(s))
	}
}
