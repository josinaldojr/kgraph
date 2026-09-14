## Why

`kgraph serve` today is single-project: it binds to one repo via `--repo` and fails unless that exact repo was previously built. There is no way to answer "which projects already have a generated graph?" — each graph lives in an opaque per-repo cache directory (`<cache>/kgraph/<sha256(repoPath)[:16]>/graph.db`), nothing enumerates them, and orphaned databases silently accumulate when repos are moved, renamed, or deleted (a real orphan from a scratch repo was found on this machine during exploration). The user must remember every built repo path and re-type it to serve anything.

## What Changes

- `kgraph serve` without `--repo` becomes **hub mode** by default: it discovers every built graph in the cache, serves a project-picker page at `/`, and lazily serves the existing graph viewer under `/p/<key>/` with per-project API routes and per-project SSE streams.
- `kgraph serve --repo <path>` keeps today's single-project behavior exactly (including the "run `kgraph build` first" failure when nothing was built) — no breaking change for existing usage.
- New `kgraph serve --pick`: interactive terminal picker (numbered list, no new TUI dependency) that selects one built project and serves it in single-project mode.
- New `kgraph projects`: non-interactive CLI listing of every built graph — display name, repo path, last build time, node/edge counts, and freshness relative to the repo's git HEAD (up to date / behind / repo missing).
- New `kgraph prune`: deletes abandoned graph databases — those whose recorded `repo_path` no longer exists on disk, plus databases with no `build_meta` row (created but never built). Interactive confirmation by default, with `--dry-run` and `--yes` flags. Stale-but-existing repos are flagged by `projects`, never pruned.
- Project freshness signals: compare `build_meta.last_commit` against the repo's current git HEAD (via existing `internal/gitutil`) in the hub picker, `/api/projects`, and `kgraph projects`.

## Capabilities

### New Capabilities
<!-- None: every behavior lands in an existing capability. -->

### Modified Capabilities
- `kgraph-cli`: `serve` gains hub mode as its default plus `--pick`; new `projects` and `prune` commands.
- `graph-visualization`: adds project selection — hub picker page, per-project viewer routes, lazy per-project loading, and per-project live refresh.
- `graph-storage`: adds discovery/enumeration of built graph databases from the cache directory, including handling of orphaned (repo missing) and never-built databases.

## Impact

- **Code (kgraph repo)**: `cmd/kgraph` (serve rewiring, new `projects`/`prune` commands, `--pick` flag); `internal/server` (hub orchestration: project registry, lazy per-project runtimes, new routes, picker page in the embedded frontend); `internal/store` (read-only cache scan, `build_meta`/count queries usable without knowing the repo path up front); `internal/gitutil` (reused for freshness, no new dependency).
- **Frontend**: embedded static assets gain a picker view; the existing viewer becomes project-scoped (`/p/<key>/`) while single-project mode keeps the viewer at `/`.
- **Dependencies**: none added (plain stdin prompt for the terminal picker; existing cobra/modernc-sqlite).
- **Compatibility**: fully backward compatible for `serve --repo`; no schema migration (existing `build_meta` rows carry everything discovery needs).
- **Platform note**: on Windows, `prune` cannot delete a database file still held open by a running `serve`; it reports the conflict and continues instead of failing the whole run.
