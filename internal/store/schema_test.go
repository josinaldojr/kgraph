package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// oldEdgesSchema mimics the edges table shape from before the confidence
// column was added, so the migration test exercises Open() against a
// database created by an older kgraph version.
const oldEdgesSchema = `
CREATE TABLE nodes (
	id         TEXT PRIMARY KEY,
	type       TEXT NOT NULL,
	file       TEXT,
	line_start INTEGER,
	line_end   INTEGER,
	signature  TEXT,
	hash       TEXT NOT NULL,
	properties TEXT NOT NULL DEFAULT '{}',
	updated_at INTEGER NOT NULL
);
CREATE TABLE edges (
	id         TEXT PRIMARY KEY,
	type       TEXT NOT NULL,
	src_id     TEXT NOT NULL,
	dst_id     TEXT NOT NULL,
	properties TEXT NOT NULL DEFAULT '{}'
);
`

func TestOpenMigratesOldDatabaseAddsConfidenceColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := db.Exec(oldEdgesSchema); err != nil {
		t.Fatalf("applying old schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO nodes (id, type, file, line_start, line_end, signature, hash, updated_at) VALUES ('a', 'Package', '', 0, 0, '', 'h', 0), ('b', 'Package', '', 0, 0, '', 'h', 0)`); err != nil {
		t.Fatalf("seeding nodes: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO edges (id, type, src_id, dst_id) VALUES ('e1', 'imports', 'a', 'b')`); err != nil {
		t.Fatalf("seeding old edge row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("closing seed connection: %v", err)
	}

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() on pre-migration database error = %v", err)
	}
	defer s.Close()

	has, err := hasColumn(s.db, "edges", "confidence")
	if err != nil {
		t.Fatalf("hasColumn() error = %v", err)
	}
	if !has {
		t.Fatal("expected confidence column to exist after Open()")
	}

	g, err := s.LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph() error = %v", err)
	}
	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Confidence != "EXTRACTED" {
		t.Errorf("expected pre-existing edge to default to EXTRACTED confidence, got %q", edges[0].Confidence)
	}
}
