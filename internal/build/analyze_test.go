package build

import (
	"os"
	"path/filepath"
	"testing"
)

// syntheticRepo writes a minimal, self-contained Go module to a temp dir
// so build pipeline tests don't depend on an external sibling checkout —
// unlike realRepoTarget, this always runs.
func syntheticRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module fixture\n\ngo 1.21\n")
	mustWrite(t, filepath.Join(dir, "main.go"), `package fixture

// NOTE: Greet is kept trivial on purpose, see design doc.
func Greet(name string) string {
	return "hello " + name
}

func Shout(name string) string {
	return Greet(name)
}
`)
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestBuildRunProducesAnalyticsMetadata(t *testing.T) {
	repo := syntheticRepo(t)
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	res, err := Run(repo, dbPath)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Graph.NodeCount() == 0 {
		t.Fatal("expected at least one node extracted")
	}

	for _, n := range res.Graph.Nodes() {
		if _, ok := n.Properties["degree"]; !ok {
			t.Errorf("expected node %s to have a degree property after build", n.ID)
		}
		if n.Community() == -1 {
			t.Errorf("expected node %s to have a community assigned after build", n.ID)
		}
	}
	for _, e := range res.Graph.Edges() {
		if e.Confidence == "" {
			t.Errorf("expected edge %s to have a confidence value after build", e.ID)
		}
	}

	foundRationale := false
	for _, n := range res.Graph.Nodes() {
		if kind, _ := n.Properties["kind"].(string); kind == "NOTE" {
			foundRationale = true
		}
	}
	if !foundRationale {
		t.Error("expected the NOTE comment in the fixture to produce a Rationale node")
	}
}

func TestBuildRunNoAnalyzeSkipsAnalytics(t *testing.T) {
	repo := syntheticRepo(t)
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	res, err := Run(repo, dbPath, Options{NoAnalyze: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, n := range res.Graph.Nodes() {
		if _, ok := n.Properties["degree"]; ok {
			t.Errorf("expected --no-analyze to skip degree computation, but node %s has one", n.ID)
		}
	}
	for _, e := range res.Graph.Edges() {
		if e.Confidence == "" {
			t.Errorf("expected confidence to still be set with --no-analyze (enrich always runs), edge %s has none", e.ID)
		}
	}
}
