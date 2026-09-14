## Context

`kgraph` persists each repo's graph in an isolated SQLite database at `<UserCacheDir>/kgraph/<sha256(absRepoPath)[:16]>/graph.db` — deliberately external to the analyzed repo. The hash directory name is opaque and not reversible, but every database records its own `repo_path`, `last_commit`, and `last_build_at` in the `build_meta` table. Today nothing enumerates these databases: `kgraph serve` is hard-bound to one repo via `--repo` (failing without a prior build), so a machine with several built graphs has no entry point that shows them, no way to pick among them, and no cleanup path for databases whose repos were moved or deleted (an orphan from a temp scratch repo confirmed this happens in practice).

The serving stack is already decomposed in a way that favors this change: `ReadOnlyStore` (read-only SQLite access, `query_only` pragma, built to coexist with an external writer), `Poller` (watches `build_meta.last_build_at`, reloads on change), `GraphService` (atomic snapshot holder), `Broadcaster` (SSE fan-out), and a small embedded static frontend. All of these are per-project instances today; the hub is primarily orchestration of N of them plus a discovery layer.

## Goals / Non-Goals

**Goals:**
- One entry point (`kgraph serve`, no flags) that lists every built graph and lets the user choose which to visualize.
- Equivalent listing in the terminal (`kgraph projects`) and an interactive single-project serve (`kgraph serve --pick`).
- Cleanup of abandoned databases (`kgraph prune`) with conservative, confirmation-gated semantics.
- Freshness signals (up to date / behind HEAD / repo missing) wherever projects are listed.
- `kgraph serve --repo <path>` behaves exactly as it does today.

**Non-Goals:**
- Cross-repo graph queries or federated visualization (each project is still viewed in isolation).
- Building/rebuilding graphs from the web UI (serve stays read-only; builds remain a separate CLI process).
- Deduplicating databases that point at the same repo under different path spellings (both are listed; dedup is future work).
- Authentication/multi-user concerns (server stays localhost-oriented).
- Eviction of idle per-project runtimes (accepted trade-off, see Decisions).

## Decisions

### Decision 1: Discovery = cache scan + `build_meta` read, in `internal/store`
A new discovery function (e.g. `store.DiscoverProjects()`) globs `<UserCacheDir>/kgraph/*/graph.db`, opens each database read-only, reads its `build_meta` row(s) plus cheap `COUNT(*)` over `nodes`/`edges`, then closes it. Unreadable/corrupt databases are returned as entries with an `unreadable` status rather than failing the whole scan. This lives in `internal/store` (it is storage-shape knowledge) and is shared by the hub server, `kgraph projects`, and `kgraph prune`.

*Alternatives considered:* a central index/manifest file written at build time (rejected: adds a second source of truth that can drift from the per-repo DBs and changes `build`'s write path); reverse-mapping hashes to paths (impossible — the hash is one-way).

### Decision 2: Project identity = the 16-hex cache directory name
URLs and the runtime registry key projects by their existing directory name (e.g. `/p/2b3915b6721d9932/`). Display name is `filepath.Base(repo_path)` with the full path as subtitle.

*Alternatives considered:* URL-encoding `repo_path` (rejected: contains `\`, `:`, spaces; ugly and fragile); repo basename as key (rejected: not unique across locations). The hash key is already unique, stable, and URL-safe.

### Decision 3: Hub mode = lazy per-project runtimes, no eviction
The hub keeps `map[key]*ProjectRuntime` guarded by a mutex. A runtime (`ReadOnlyStore` + `Poller` + `GraphService` + `Broadcaster`, with its own goroutine tied to the server context) is created on the first request targeting that project and lives until server shutdown. `/api/projects` itself never opens runtimes — it only runs the cheap discovery scan.

*Alternatives considered:* load all graphs at startup (rejected: O(N) memory and slow startup for a machine with many repos); single active project with a server-side "current selection" that switches on demand (rejected: multiple browser tabs would fight over one global slot, and switching would destroy state another tab is using).

Eviction (dropping runtimes idle for some TTL) is deliberately deferred: with personal-scale project counts, one poller goroutine plus one open read-only connection per visited project is trivial, and eviction adds snapshot-reload latency and lifecycle bugs.

### Decision 4: Routing — per-project API prefix; legacy routes untouched in single mode
Hub mode serves:
```
GET /                                  picker page
GET /api/projects                      project list (discovery scan)
GET /p/<key>/                          viewer page for project <key>
GET /api/projects/<key>/graph          (also: /graph/local, /node, /search)
GET /api/projects/<key>/events         per-project SSE
GET /static/...                        embedded assets (unchanged)
```
Single-project mode (`--repo`, and the result of `--pick`) keeps today's exact surface: viewer at `/`, `/api/graph`, `/api/node`, `/api/search`, `/api/graph/local`, `/events`. Internally both modes reuse the same handler implementations; single mode wires them to one pinned runtime, hub mode resolves the runtime from the `<key>` path segment.

### Decision 5: Frontend configuration via server-rendered bootstrap
`index.html` becomes a small Go template; the server injects a config object (`mode: "hub" | "single"`, current project key when single, API base path). The picker is a plain DOM view over `/api/projects`; the existing canvas viewer code is unchanged except for computing its API URLs from the injected base. No build step, no framework — consistent with the web-viewer change's Decision 4.

### Decision 6: Freshness = `last_commit` vs `git rev-parse HEAD`, best-effort
For each discovered project whose `repo_path` exists, discovery runs `git -C <repo> rev-parse HEAD` (via existing `internal/gitutil`, short timeout) and compares to `build_meta.last_commit`: equal → `current`, different → `stale`, git fails/absent → `unknown`; nonexistent path → `missing`. Exact ancestor checking (`merge-base`) would be more precise but costs an extra git call per project; string inequality is good enough for a "needs `kgraph update`?" hint.

### Decision 7: `prune` candidates and safety
Candidates, in order of detection: (a) databases with **no** `build_meta` row (created but never built); (b) databases whose recorded `repo_path` no longer exists on disk (`os.Stat`). Databases whose repo exists are never candidates regardless of staleness. Flow: discover candidates → print them with size and reason → interactive y/N confirmation per candidate (or for the batch). Flags: `--dry-run` (list only, no prompt), `--yes` (skip confirmation). Deletion removes the whole `<key>/` directory (`graph.db`, `-wal`, `-shm`). On Windows a running `serve` may hold the file open; a delete failure is reported as "in use, skipped" and prune continues with remaining candidates, exiting non-zero only if every candidate failed.

### Decision 8: `--pick` is a plain stdin prompt
`kgraph serve --pick` runs discovery, prints a numbered list (name, path, freshness), reads a number from stdin, then serves that project in single-project mode. No TUI dependency. If stdin is not a terminal (piped/CI), `--pick` fails with a clear message to use `--repo` instead.

## Risks / Trade-offs

- [Discovery opens every database on each `/api/projects` call] → At personal scale (single-digit to low tens of DBs) each scan is a few ms per DB with the connection closed immediately after; caching can be added later if it ever shows up.
- [False-positive orphans: repo on a disconnected external drive looks "missing"] → Prune is confirmation-gated and shows path + size + reason; `--dry-run` first; nothing is deleted without explicit consent.
- [Windows file locks block pruning a DB that a running hub has open] → Prune reports the conflict per-candidate and continues; guidance printed to stop `serve` first.
- [Lazy runtimes are never evicted] → Accepted: one poller goroutine + one read-only SQLite connection per visited project; revisit only if project counts grow far beyond personal use.
- [Two DBs for the same repo (different path spellings/case)] → Both listed; potentially confusing but honest. Deduplication deferred.
- [Freshness check runs git per project] → Bounded by project count, short timeout, failure degrades to `unknown` rather than breaking discovery.
- [Serve stays read-only] → Hub, picker, and prune never write to any graph database; `prune` deletes whole database files, which is a filesystem operation, not a write through the store.

## Migration Plan

No data migration: existing databases already carry everything discovery needs (`build_meta.repo_path`, `last_commit`, `last_build_at`). No schema change. Rollback is reverting the binary; `serve` itself never mutates data. The only destructive operation (`prune`) is new, opt-in, and confirmation-gated.

## Open Questions

None blocking. Deferred-by-decision items (runtime eviction, same-repo DB deduplication, `merge-base`-precise freshness) are recorded above and can become follow-up changes if they start hurting.
