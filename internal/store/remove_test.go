package store

import (
	"os"
	"path/filepath"
	"testing"
)

// buildRemovableProject creates a project directory with graph.db plus fake
// -wal/-shm sidecars so tests can assert the whole directory goes away.
func buildRemovableProject(t *testing.T, root, key string) string {
	t.Helper()
	dbPath := buildFixtureProject(t, root, key, "/repo/"+key, "c1", 2)
	for _, sidecar := range []string{dbPath + "-wal", dbPath + "-shm"} {
		if err := os.WriteFile(sidecar, []byte("sidecar"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", sidecar, err)
		}
	}
	return filepath.Dir(dbPath)
}

func TestRemoveProjectDeletesDirectoryIncludingSidecars(t *testing.T) {
	root := t.TempDir()
	dir := buildRemovableProject(t, root, "aaaaaaaaaaaaaaaa")

	if err := RemoveProjectFrom(root, "aaaaaaaaaaaaaaaa"); err != nil {
		t.Fatalf("RemoveProjectFrom() error = %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected the project directory to be gone, Stat err = %v", err)
	}
}

func TestRemoveProjectLeavesSiblingsIntact(t *testing.T) {
	root := t.TempDir()
	buildRemovableProject(t, root, "aaaaaaaaaaaaaaaa")
	sibling := buildRemovableProject(t, root, "bbbbbbbbbbbbbbbb")

	if err := RemoveProjectFrom(root, "aaaaaaaaaaaaaaaa"); err != nil {
		t.Fatalf("RemoveProjectFrom() error = %v", err)
	}

	// The sibling's directory, database, and sidecars must all survive —
	// and stay readable.
	for _, f := range []string{sibling, filepath.Join(sibling, "graph.db"), filepath.Join(sibling, "graph.db-wal"), filepath.Join(sibling, "graph.db-shm")} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("sibling file %s should survive removal of another project: %v", f, err)
		}
	}
	projects, err := DiscoverProjectsWith(root)
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() error = %v", err)
	}
	if len(projects) != 1 || projects[0].Key != "bbbbbbbbbbbbbbbb" || projects[0].Status != StatusBuilt {
		t.Errorf("expected only the sibling to remain discoverable, got %+v", projects)
	}
}

func TestRemoveProjectRejectsPathTraversalKeys(t *testing.T) {
	// Place a victim directory next to the cache root so a traversal-shaped
	// key would reach it if the guard failed.
	parent := t.TempDir()
	root := filepath.Join(parent, "cache")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	victim := filepath.Join(parent, "victim")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(victim, "important.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	for _, key := range []string{
		"..",
		"../victim",
		"..\\victim",
		"aaaaaaaaaaaaaaaa/../victim",
		"aaaaaaaaaaaaaaaa/../../victim",
	} {
		if err := RemoveProjectFrom(root, key); err == nil {
			t.Errorf("RemoveProjectFrom(%q) expected an error, got nil", key)
		}
	}
	if _, err := os.Stat(filepath.Join(victim, "important.txt")); err != nil {
		t.Errorf("a traversal-shaped key must never touch other directories: %v", err)
	}
}

func TestRemoveProjectMissingDirectoryIsNotAnError(t *testing.T) {
	if err := RemoveProjectFrom(t.TempDir(), "aaaaaaaaaaaaaaaa"); err != nil {
		t.Errorf("removing a nonexistent project should be a no-op, got %v", err)
	}
}
