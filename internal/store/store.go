// Package store persists the code knowledge graph to SQLite
// (modernc.org/sqlite — no cgo, per design.md's decision to keep kgraph's
// build cgo-free the way knowledge-cli's own store is).
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database holding the persisted graph, summaries,
// and build metadata for a single analyzed repository.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at dbPath and
// applies the schema. The caller must Close the returned Store.
func Open(dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: creating %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("store: opening %s: %w", dbPath, err)
	}
	// A single-writer CLI tool doesn't need connection pooling; one
	// connection avoids SQLITE_BUSY from concurrent writers within the
	// same process. See design.md's "single-process, no lock manager"
	// trade-off.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: applying schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
