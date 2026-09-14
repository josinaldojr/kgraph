package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josinaldojr/kgraph/internal/store"
)

// realRepoTarget is the checkpoint validation target for task 5.2: a real
// Go repository elsewhere in the workspace, not a synthetic fixture. It's
// skipped automatically if the sibling checkout isn't present (e.g. CI
// running kgraph on its own).
func realRepoTarget(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "anti-fraudeiro")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("real-repo checkpoint target not present at %s, skipping: %v", path, err)
	}
	return path
}

func TestBuildRunAgainstRealRepo(t *testing.T) {
	repo := realRepoTarget(t)
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	res, err := Run(repo, dbPath)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, w := range res.Warnings {
		t.Logf("warning: %s", w)
	}

	if res.Graph.NodeCount() == 0 {
		t.Fatal("expected at least one node extracted from a real repo, got 0")
	}
	t.Logf("nodes=%d edges=%d written(nodes=%d edges=%d) commit=%s",
		res.Graph.NodeCount(), res.Graph.EdgeCount(), res.Stats.NodesWritten, res.Stats.EdgesWritten, res.Commit)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("re-opening store: %v", err)
	}
	defer s.Close()
	loaded, err := s.LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph() error = %v", err)
	}
	if loaded.NodeCount() != res.Graph.NodeCount() || loaded.EdgeCount() != res.Graph.EdgeCount() {
		t.Errorf("persisted graph mismatch: loaded %d/%d, extracted %d/%d",
			loaded.NodeCount(), loaded.EdgeCount(), res.Graph.NodeCount(), res.Graph.EdgeCount())
	}
}

func TestBuildRunIsIdempotentOnRealRepo(t *testing.T) {
	repo := realRepoTarget(t)
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	if _, err := Run(repo, dbPath); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	second, err := Run(repo, dbPath)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	if second.Stats.NodesWritten != 0 {
		t.Errorf("expected zero node writes on unchanged rebuild, got %d (unchanged=%d)",
			second.Stats.NodesWritten, second.Stats.NodesUnchanged)
	}
	if second.Stats.EdgesWritten != 0 {
		t.Errorf("expected zero edge writes on unchanged rebuild, got %d (unchanged=%d)",
			second.Stats.EdgesWritten, second.Stats.EdgesUnchanged)
	}
}
