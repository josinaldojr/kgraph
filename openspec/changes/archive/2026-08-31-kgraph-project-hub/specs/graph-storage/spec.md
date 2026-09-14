## ADDED Requirements

### Requirement: Discovery of built graph databases
The system SHALL enumerate every graph database under the per-user cache directory (`<UserCacheDir>/kgraph/<key>/graph.db`) by opening each database read-only and reading its build metadata, producing for each entry its cache key, recorded repository path, last commit, last build time, and node and edge counts. Discovery SHALL NOT write to any database, and SHALL tolerate unreadable or corrupt databases by reporting them as unreadable entries rather than failing the whole enumeration. A database that contains no `build_meta` row SHALL be reported as never built.

#### Scenario: Enumerating built projects
- **WHEN** discovery runs on a machine where two repositories have been built
- **THEN** it returns two entries, each carrying its cache key, recorded repository path, last commit, last build time, and node/edge counts

#### Scenario: Unreadable database does not break discovery
- **WHEN** one database file in the cache directory is corrupt or otherwise unreadable
- **THEN** discovery returns an entry for it marked unreadable and still returns every other database's entry

#### Scenario: Never-built database is reported
- **WHEN** the cache directory contains a database with schema but no `build_meta` row
- **THEN** discovery returns an entry for it marked as never built

#### Scenario: Discovery is read-only
- **WHEN** discovery runs over every database in the cache directory
- **THEN** no database file is created, modified, or deleted by the enumeration itself

### Requirement: Graph freshness status
The system SHALL determine a freshness status for each discovered database by comparing its recorded last commit against the current git HEAD of its recorded repository path: up to date when they match, stale when the repository's HEAD differs, missing when the recorded repository path does not exist on disk, and unknown when git status cannot be determined (e.g. the path is not a git repository or git fails). Freshness determination SHALL be best-effort and SHALL NOT fail discovery when a git lookup fails.

#### Scenario: Build matches HEAD
- **WHEN** a database's recorded last commit equals its repository's current git HEAD
- **THEN** the entry's freshness status is up to date

#### Scenario: Repository moved on
- **WHEN** a database's recorded last commit differs from its repository's current git HEAD
- **THEN** the entry's freshness status is stale

#### Scenario: Repository path disappeared
- **WHEN** a database's recorded repository path does not exist on disk
- **THEN** the entry's freshness status is missing, taking precedence over any git comparison

#### Scenario: Git status unavailable
- **WHEN** the recorded repository path exists but its git HEAD cannot be determined
- **THEN** the entry's freshness status is unknown and discovery still succeeds

### Requirement: Deletion of abandoned databases
The system SHALL support deleting a discovered graph database identified by cache key, removing the project's entire cache directory (the SQLite database file plus its WAL and SHM sidecars) and affecting no other database. Deletion SHALL only be reachable through explicit caller intent (the `prune` command flow), never as a side effect of discovery or serving.

#### Scenario: Deleting an orphaned database
- **WHEN** deletion is requested for a cache key whose recorded repository path no longer exists
- **THEN** that key's cache directory including WAL/SHM files is removed from disk

#### Scenario: Other databases untouched
- **WHEN** deletion removes one cache key's directory
- **THEN** every other discovered database directory remains intact and readable
