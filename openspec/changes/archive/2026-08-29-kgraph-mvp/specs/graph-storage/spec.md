## ADDED Requirements

### Requirement: In-memory graph representation
The system SHALL represent the extracted graph in memory as node and adjacency maps (no external graph library), exposing lookups for a node by ID and for a node's outgoing and incoming edges.

#### Scenario: Adjacency lookup
- **WHEN** a caller requests the edges for a given node ID
- **THEN** the system returns both outgoing edges (where the node is the source) and incoming edges (where the node is the destination)

### Requirement: SQLite persistence
The system SHALL persist nodes and edges to a SQLite database (`modernc.org/sqlite`, no cgo) using a `nodes` table and an `edges` table, each storing a `type` column and a JSON `properties` column for type-specific attributes.

#### Scenario: Node round-trip
- **WHEN** the graph is persisted after a build and then reloaded
- **THEN** every node's type, file, line range, signature, hash, and properties match what was extracted

### Requirement: Idempotent persistence by content hash
The system SHALL key persistence writes by each node's content hash so that persisting an unchanged node does not update its stored row.

#### Scenario: No-op write on unchanged node
- **WHEN** persistence runs for a node whose computed hash matches the hash already stored for that node ID
- **THEN** the stored row (including `updated_at`) is left unmodified
