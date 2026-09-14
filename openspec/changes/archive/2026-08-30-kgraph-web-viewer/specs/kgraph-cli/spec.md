## ADDED Requirements

### Requirement: serve command
The CLI SHALL provide `kgraph serve [--addr <host:port>]` which starts a long-running, read-only HTTP server exposing the graph-visualization capability over the database resolved from `--repo`/`--db`, defaulting `--addr` to a localhost address when not specified, and SHALL NOT perform any write to the database from within `serve` itself.

#### Scenario: Starting the server with defaults
- **WHEN** `kgraph serve` is run against a previously built repository with no `--addr` given
- **THEN** the command starts an HTTP server bound to localhost and continues running until stopped, without modifying the database

#### Scenario: Serving without a prior build
- **WHEN** `kgraph serve` is run against a `--db` path that has never been built
- **THEN** the command fails with a clear error rather than starting a server over an empty or nonexistent graph

#### Scenario: Serving while another process writes
- **WHEN** `kgraph build` or `kgraph update` runs against the same database while `kgraph serve` is already running
- **THEN** `serve` continues responding to requests throughout, and subsequently reflects the new data per the graph-visualization live-refresh requirement
