# code-graph-extraction

## Purpose

TBD - extracted from kgraph-mvp change. Parses a target Go repository's source, ORM tags, and SQL migrations into typed graph nodes and edges (packages, structs, interfaces, functions/methods, fields, tables, columns) with idempotent, hash-based re-extraction.

## Requirements

### Requirement: Declaration extraction
The system SHALL parse each Go source file in the target repository using `go/parser` and produce typed nodes for packages, structs, interfaces, functions/methods, and struct fields, each carrying file path, start/end line, signature, and a content hash.

#### Scenario: Struct with fields
- **WHEN** a source file declares a struct with one or more fields
- **THEN** the system creates a `Struct` node and a `Field` node per struct field, connected by `has_field` edges

#### Scenario: Method with receiver
- **WHEN** a source file declares a function with a receiver (a method)
- **THEN** the system creates a `Function` node linked to its receiver's `Struct` node via a `has_method` edge

### Requirement: Call edge detection
The system SHALL detect direct function/method calls made from within a function body by walking its AST, and represent each as a `calls` edge from the caller `Function` node to the callee `Function` node, without requiring full type inference.

#### Scenario: Direct call between two functions
- **WHEN** function A's body contains a call expression invoking function B (resolvable via `go/packages` package-level type info)
- **THEN** a `calls` edge from A to B is created in the graph

### Requirement: ORM tag mapping
The system SHALL detect `gorm` and `db` struct tags on struct fields and derive `Table`/`Column` nodes and `maps_to_table`/`references_fk` edges from them.

#### Scenario: Tagged struct maps to a table
- **WHEN** a struct's fields carry `gorm` or `db` tags identifying column names
- **THEN** the system creates a `Table` node, one `Column` node per tagged field, and a `maps_to_table` edge from the `Struct` node to the `Table` node

#### Scenario: Foreign key tag
- **WHEN** a tagged field's ORM annotation identifies a foreign-key relationship to another mapped struct/table
- **THEN** the system creates a `references_fk` edge from the owning `Column` node to the referenced `Table`/`Column` node

### Requirement: SQL migration reconciliation
The system SHALL parse `.sql` migration files present in the repository and reconcile their schema definitions with struct-tag-derived `Table`/`Column` nodes, with migration-derived facts taking precedence on conflict.

#### Scenario: Migration overrides struct tag
- **WHEN** a `.sql` migration defines a column differently than a struct's ORM tag (e.g. renamed or dropped)
- **THEN** the resulting `Table`/`Column` nodes reflect the migration's definition, and the conflicting struct-tag-derived fact is retained in `properties` rather than silently discarded

### Requirement: Idempotent, error-free extraction
The system SHALL complete extraction over a real Go repository without error, and re-running extraction on an unchanged repository SHALL produce no changes to node/edge content hashes.

#### Scenario: Rebuild with no source changes
- **WHEN** `build` is run a second time against a repository with no source changes since the first run
- **THEN** every node's content hash is identical to the prior run and no node/edge rows are modified
