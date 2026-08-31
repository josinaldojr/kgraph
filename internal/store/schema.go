package store

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
