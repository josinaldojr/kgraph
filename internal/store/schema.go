package store

import "database/sql"

// schema is applied on every Open call; every statement is idempotent
// (CREATE TABLE/INDEX IF NOT EXISTS), so opening an existing database is
// cheap and safe.
const schema = `
CREATE TABLE IF NOT EXISTS nodes (
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

CREATE TABLE IF NOT EXISTS edges (
	id         TEXT PRIMARY KEY,
	type       TEXT NOT NULL,
	src_id     TEXT NOT NULL,
	dst_id     TEXT NOT NULL,
	confidence TEXT NOT NULL DEFAULT 'EXTRACTED',
	properties TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_edges_src ON edges(src_id);
CREATE INDEX IF NOT EXISTS idx_edges_dst ON edges(dst_id);

CREATE TABLE IF NOT EXISTS summaries (
	node_id TEXT NOT NULL,
	level   TEXT NOT NULL, -- 'node' | 'file' | 'module'
	hash    TEXT NOT NULL,
	summary TEXT,
	model   TEXT, -- identifies what produced the summary (e.g. a provider/session id)
	stale   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (node_id, level)
);

CREATE TABLE IF NOT EXISTS build_meta (
	repo_path     TEXT PRIMARY KEY,
	last_commit   TEXT,
	last_build_at INTEGER
);
`

// migrate applies schema changes that CREATE TABLE IF NOT EXISTS can't
// express, i.e. columns added to a table that may already exist from a
// database created by an older kgraph version. Each migration checks
// whether it's needed before applying, so it's safe to run on every Open.
func migrate(db *sql.DB) error {
	hasCol, err := hasColumn(db, "edges", "confidence")
	if err != nil {
		return err
	}
	if !hasCol {
		if _, err := db.Exec(`ALTER TABLE edges ADD COLUMN confidence TEXT NOT NULL DEFAULT 'EXTRACTED'`); err != nil {
			return err
		}
	}
	return nil
}

// hasColumn reports whether table has a column named col.
func hasColumn(db *sql.DB, table, col string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}
