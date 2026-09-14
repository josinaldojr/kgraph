## MODIFIED Requirements

### Requirement: serve command
The CLI SHALL provide `kgraph serve [--addr <host:port>] [--repo <path>] [--db <path>] [--pick]` which starts a long-running, read-only HTTP server, defaulting `--addr` to a localhost address when not specified, and SHALL NOT perform any write to any graph database from within `serve` itself in any mode. Without `--repo` (and without `--pick`), `serve` runs in **hub mode**: it discovers every built graph database in the per-user cache directory and serves the project-selection experience over all of them. With `--repo` (and optionally `--db`), `serve` runs in single-project mode exactly as before: bound to that repository's database and failing with a clear error when no build is recorded for it. With `--pick`, `serve` presents an interactive terminal picker over the discovered projects and then serves the chosen project in single-project mode.

#### Scenario: Starting hub mode with defaults
- **WHEN** `kgraph serve` is run with no `--repo`, `--db`, or `--pick` and no `--addr`
- **THEN** the command starts an HTTP server bound to localhost serving the project hub over every discovered built graph, continues running until stopped, and does not modify any database

#### Scenario: Starting single-project mode
- **WHEN** `kgraph serve` is run with `--repo` pointing at a previously built repository and no `--addr`
- **THEN** the command starts an HTTP server bound to localhost serving that repository's graph directly (no project picker), and continues running until stopped, without modifying the database

#### Scenario: Serving a repo without a prior build
- **WHEN** `kgraph serve --repo <path>` is run against a `--db` path that has never been built
- **THEN** the command fails with a clear error rather than starting a server over an empty or nonexistent graph

#### Scenario: Serving while another process writes
- **WHEN** `kgraph build` or `kgraph update` runs against a database that a running `kgraph serve` is displaying (in either mode)
- **THEN** `serve` continues responding to requests throughout, and subsequently reflects the new data per the graph-visualization live-refresh requirement

#### Scenario: Interactive pick selects a project to serve
- **WHEN** `kgraph serve --pick` is run with an interactive terminal and the user selects one of the listed discovered projects
- **THEN** `serve` starts in single-project mode bound to the selected project's database

#### Scenario: Pick requires an interactive terminal
- **WHEN** `kgraph serve --pick` is run with stdin that is not a terminal (e.g. piped or redirected)
- **THEN** the command fails with a clear error directing the user to `--repo` instead, and does not start a server

## ADDED Requirements

### Requirement: projects command
The CLI SHALL provide `kgraph projects` which discovers every graph database in the per-user cache directory and prints one entry per database — display name (repository basename), full recorded repository path, last build time, node and edge counts, and freshness status (up to date / behind HEAD / repository missing / never built / unreadable) — and SHALL exit with status 0 with an explicit "no built graphs" message when nothing has been discovered.

#### Scenario: Listing multiple built projects
- **WHEN** `kgraph projects` is run on a machine where two repositories have been built
- **THEN** the command prints both entries with their paths, counts, last build times, and freshness, and exits 0

#### Scenario: Listing when nothing is built
- **WHEN** `kgraph projects` is run on a machine with no built graph databases
- **THEN** the command prints an explicit message that no graphs have been built yet and exits 0

#### Scenario: Orphaned database is flagged
- **WHEN** a discovered database's recorded repository path no longer exists on disk
- **THEN** that entry is still listed, clearly marked as repository missing, so the user can see it is a prune candidate

### Requirement: prune command
The CLI SHALL provide `kgraph prune [--dry-run] [--yes]` which deletes abandoned graph databases — databases with no `build_meta` row (created but never built) or whose recorded repository path no longer exists on disk — and SHALL NOT treat a database as a candidate while its recorded repository path exists, regardless of how stale its build is. Deletion removes the project's entire cache directory (database file plus WAL/SHM sidecars). Without `--yes`, prune SHALL list the candidates with size and reason and require explicit confirmation before deleting anything; with `--dry-run` it SHALL only list candidates without prompting or deleting. A candidate that cannot be deleted (e.g. its file is held open by a running `kgraph serve` on Windows) SHALL be reported as skipped with the reason, and pruning SHALL continue with the remaining candidates.

#### Scenario: Orphaned database is deleted after confirmation
- **WHEN** `kgraph prune` is run with one discovered database whose recorded repository path no longer exists, and the user confirms
- **THEN** that database's cache directory (including WAL/SHM files) is deleted and every other discovered database is untouched

#### Scenario: Stale build of an existing repo is never pruned
- **WHEN** `kgraph prune` is run and a discovered database's repository exists but its `last_commit` is behind the repository's current git HEAD
- **THEN** that database is not offered as a candidate and is not deleted

#### Scenario: Dry run changes nothing
- **WHEN** `kgraph prune --dry-run` is run with one or more candidates
- **THEN** the candidates are listed with size and reason, no confirmation prompt appears, and no file is deleted

#### Scenario: Locked database is skipped, not fatal
- **WHEN** `kgraph prune --yes` attempts to delete a candidate whose database file is held open by another process and the operating system refuses the deletion
- **THEN** prune reports that candidate as skipped with a reason, continues with the remaining candidates, and exits non-zero only if every candidate failed

#### Scenario: Nothing to prune
- **WHEN** `kgraph prune` is run and every discovered database belongs to an existing repository and has a `build_meta` row
- **THEN** the command reports that there is nothing to prune and exits 0 without prompting
