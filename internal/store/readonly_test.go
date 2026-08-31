package store

import (
	"path/filepath"
	"testing"
)

func TestOpenReadOnlyMissingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "graph.db")
	if _, err := OpenReadOnly(dbPath); err == nil {
		t.Fatal("expected an error opening a database that was never built")
	}
}

func TestReadOnlyStoreSeesWriterData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "graph.db")
	rw, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer rw.Close()

	g := sampleGraph()
	if _, err := rw.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}
	if err := rw.SetLastCommit("/repo", "abc123"); err != nil {
		t.Fatalf("SetLastCommit() error = %v", err)
	}
	if err := rw.UpsertSummary("demo.Foo()", "node", "h2", "Does foo.", "test-model"); err != nil {
		t.Fatalf("UpsertSummary() error = %v", err)
	}

	ro, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly() error = %v", err)
	}
	defer ro.Close()

	loaded, err := ro.LoadGraph()
	if err != nil {
		t.Fatalf("ReadOnlyStore.LoadGraph() error = %v", err)
	}
	if loaded.NodeCount() != g.NodeCount() || loaded.EdgeCount() != g.EdgeCount() {
		t.Fatalf("expected %d nodes/%d edges via read-only handle, got %d/%d",
			g.NodeCount(), g.EdgeCount(), loaded.NodeCount(), loaded.EdgeCount())
	}

	ts, ok, err := ro.LastBuildAt("/repo")
	if err != nil || !ok || ts == 0 {
		t.Fatalf("expected a recorded last_build_at via read-only handle, got ts=%d ok=%v err=%v", ts, ok, err)
	}

	rec, ok, err := ro.GetSummary("demo.Foo()", "node")
	if err != nil || !ok || rec.Summary != "Does foo." {
		t.Fatalf("expected summary via read-only handle, got %+v ok=%v err=%v", rec, ok, err)
	}
}

// TestReadOnlyStoreDuringOpenWriteTransaction confirms a ReadOnlyStore can
// still be opened and queried while a separate Store handle to the same
// file has an uncommitted write transaction open — the concurrency case
// `kgraph serve` (reader) and `kgraph build`/`update` (writer) rely on.
// WAL readers never see uncommitted rows (correct MVCC behavior, not
// tested here); what matters is that the read doesn't block or error.
func TestReadOnlyStoreDuringOpenWriteTransaction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "graph.db")
	rw, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer rw.Close()
	if _, err := rw.SaveGraph(sampleGraph()); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}

	ro, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly() error = %v", err)
	}
	defer ro.Close()

	tx, err := rw.db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO build_meta (repo_path, last_commit, last_build_at) VALUES (?, ?, ?)`,
		"/mid-tx", "deadbeef", 1); err != nil {
		t.Fatalf("mid-transaction insert error = %v", err)
	}

	if _, err := ro.LoadGraph(); err != nil {
		t.Fatalf("ReadOnlyStore.LoadGraph() while writer mid-transaction: %v", err)
	}
	if _, ok, err := ro.LastBuildAt("/mid-tx"); err != nil {
		t.Fatalf("ReadOnlyStore.LastBuildAt() while writer mid-transaction: %v", err)
	} else if ok {
		t.Fatal("expected the reader not to see the writer's uncommitted row")
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
}
