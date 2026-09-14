## 1. Discovery layer (internal/store)

- [x] 1.1 Add `ProjectInfo` type (cache key, repo path, last commit, last build time, node/edge counts, status: built / never-built / unreadable) and a `DiscoverProjects()` function that globs `<UserCacheDir>/kgraph/*/graph.db`, opens each read-only, reads `build_meta` + `COUNT(*)` over nodes/edges, closes the connection, and tolerates unreadable databases as entries instead of errors; include a way to point discovery at a custom cache root for tests
- [x] 1.2 Add freshness determination: compare `last_commit` against `git -C <repo> rev-parse HEAD` via `internal/gitutil` (short timeout), producing up-to-date / stale / missing / unknown with `missing` (path does not exist) taking precedence; git failure degrades to `unknown` without failing discovery
- [x] 1.3 Unit tests for discovery: two built fixture DBs enumerated with correct metadata; corrupt DB reported unreadable while others succeed; schema-only DB reported never-built; discovery performs no writes; freshness cases (match, behind, missing path, non-git path)

## 2. Deletion primitive (internal/store)

- [x] 2.1 Add `RemoveProject(key)` that deletes one project's entire cache directory (`graph.db`, `-wal`, `-shm`) by key, refuses keys outside the cache root, and touches no other directory; return a distinguishable error when the OS refuses deletion (e.g. file held open)
- [x] 2.2 Unit tests for RemoveProject: deletes the target directory including sidecars; leaves sibling project directories intact; rejects path-traversal-shaped keys

## 3. CLI: `kgraph projects`

- [x] 3.1 Implement `cmd_projects.go`: run discovery + freshness, print one entry per database (display name = repo basename, full path, last build time, node/edge counts, freshness flag), explicitly mark orphaned entries as prune candidates, exit 0 with an explicit "no built graphs" message when empty
- [x] 3.2 Test the output formatting/selection logic (extract the formatting into a testable function; cover built, orphaned, never-built, unreadable entries)

## 4. CLI: `kgraph prune`

- [x] 4.1 Implement `cmd_prune.go`: candidate detection from discovery (never-built OR repo path missing; existing repos never candidates), listing candidates with size and reason, interactive confirmation before deletion, `--dry-run` (list only, no prompt) and `--yes` (skip confirmation)
- [x] 4.2 Handle per-candidate deletion failure (Windows file lock): report "skipped, in use" with reason, continue with remaining candidates, exit non-zero only if every candidate failed; report "nothing to prune" and exit 0 when no candidates
- [x] 4.3 Tests: candidate selection rules (orphan yes, stale-but-existing no, never-built yes); dry-run deletes nothing; skip-and-continue behavior on a forced deletion error

## 5. CLI: `kgraph serve --pick`

- [x] 5.1 Extract a small stdin prompt helper (numbered list + read choice) shared by `--pick` and `prune` confirmation, with a detectable non-TTY failure mode
- [x] 5.2 Wire `--pick` into `cmd_serve.go`: discover, print numbered list (name, path, freshness), read selection, then serve that project in single-project mode; non-TTY stdin fails with a clear message pointing at `--repo`
- [x] 5.3 Test picker input handling (valid choice, out-of-range, non-numeric, EOF) against the extracted helper

## 6. Hub server (internal/server)

- [x] 6.1 Introduce `ProjectRuntime` (ReadOnlyStore + Poller + GraphService + Broadcaster + cancel func) and a concurrency-safe registry that creates runtimes lazily on first access for a key, rejects unknown keys, and cancels/stops all runtimes on server shutdown
- [x] 6.2 Refactor existing graph/node/search/local handlers to operate against a resolved runtime (keep handler bodies, parameterize the snapshot source) so single-project and hub modes share implementations
- [x] 6.3 Add hub routes: `GET /api/projects` (discovery scan + freshness, JSON), `GET /api/projects/<key>/graph|graph/local|node|search`, `GET /api/projects/<key>/events` (per-project SSE), `GET /p/<key>/` (viewer page); unknown key yields a clear JSON 404
- [x] 6.4 Keep single-project mode byte-compatible: with `--repo` (or a picked project) serve `/`, `/api/graph`, `/api/graph/local`, `/api/node`, `/api/search`, `/events` exactly as today, including the "run `kgraph build` first" startup failure
- [x] 6.5 Tests: registry lazy-creation and unknown-key rejection; per-project SSE isolation (rebuilding project B emits no event on project A's stream); `/api/projects` returns discovered entries without loading any graph; single-mode routes still respond as before

## 7. Frontend (embedded static assets)

- [x] 7.1 Convert `index.html` to a Go template with an injected bootstrap config (`mode: hub|single`, current project key, API base) and adapt `app.js` to build API URLs from that base (viewer behavior otherwise unchanged)
- [x] 7.2 Build the picker view (plain DOM/CSS): cards with display name, path, counts, last build (relative), freshness badge; orphaned projects flagged but selectable, unreadable listed but disabled; empty state pointing at `kgraph build`; selecting navigates to `/p/<key>/`
- [x] 7.3 Add a visible way back to the picker from a hub-mode viewer (e.g. home link in the toolbar); no such element appears in single-project mode

## 8. End-to-end verification

- [x] 8.1 Build two fixture repos (`kgraph build` in each), run `kgraph serve` hub mode: picker lists both with correct counts/freshness; open each viewer; verify search/local/detail work per project
- [x] 8.2 Live-refresh isolation check: while both viewers are open, run `kgraph update`/`build` against one repo and confirm only that project's view refreshes
- [x] 8.3 Delete or rename one fixture repo: confirm hub flags it as repository missing, its snapshot still opens, `kgraph projects` marks it, `kgraph prune` offers exactly it, and deletion removes only its directory
- [x] 8.4 Regression: `kgraph serve --repo <fixture>` behaves identically to pre-change (routes, startup error without build, live refresh); `kgraph serve --pick` selects and serves one project; `--pick` with piped stdin fails cleanly
- [x] 8.5 Run full `go test ./...` in the kgraph module and `openspec validate --change kgraph-project-hub`
