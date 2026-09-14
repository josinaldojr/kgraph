# edge-confidence

## Purpose

TBD - extracted from kgraph-graphify-evolution change. Defines the confidence tag (EXTRACTED vs INFERRED) carried by every edge in the graph, distinguishing relationships read directly from source syntax from those resolved via cross-file or heuristic inference, and how that tag is persisted and surfaced.

## Requirements

### Requirement: Every edge SHALL have a confidence tag
Each edge in the graph SHALL carry a Confidence field with one of two values: `EXTRACTED` (the relationship was read directly from source code AST or explicit syntax) or `INFERRED` (the relationship was resolved by cross-file analysis, heuristic matching, or indirect inference).

#### Scenario: Edge created from direct AST extraction
- **WHEN** the parser extracts a function call from a Go call expression, a TypeScript `import` statement, or a Python `class Foo(Bar)` inheritance
- **THEN** the resulting edge SHALL have Confidence = `EXTRACTED`

#### Scenario: Edge created from cross-file resolution
- **WHEN** the system resolves that a struct implements an interface by matching method signatures across files
- **THEN** the resulting edge SHALL have Confidence = `INFERRED`

#### Scenario: Edge created from ORM table mapping inference
- **WHEN** the Go parser infers a `maps_to_table` edge from ORM struct tags
- **THEN** the resulting edge SHALL have Confidence = `INFERRED`

### Requirement: Confidence SHALL be persisted and queryable
The confidence value SHALL be stored in the SQLite edges table and returned in all API responses that include edge data.

#### Scenario: Confidence persisted to SQLite
- **WHEN** a graph is saved to the database
- **THEN** each edge's confidence SHALL be stored in a `confidence` column on the edges table

#### Scenario: Confidence in HTTP API responses
- **WHEN** a client requests graph data via /api/graph or /api/graph/local
- **THEN** each edge in the response SHALL include its confidence value

#### Scenario: Confidence in graph.json export
- **WHEN** the graph is exported as graph.json
- **THEN** each edge object SHALL include a `confidence` field

### Requirement: Existing edges SHALL default to EXTRACTED on migration
When an existing database without a confidence column is opened, all existing edges SHALL be treated as EXTRACTED until the next rebuild populates correct values.

#### Scenario: Database migration adds confidence column
- **WHEN** kgraph opens a database created before the confidence feature
- **THEN** the schema migration SHALL add the confidence column with DEFAULT 'EXTRACTED'
- **THEN** all existing edges SHALL have confidence = 'EXTRACTED'

#### Scenario: Rebuild corrects confidence values
- **WHEN** `kgraph build` or `kgraph update` runs on a migrated database
- **THEN** edges re-extracted SHALL have their confidence set correctly (EXTRACTED or INFERRED based on extraction method)
