## MODIFIED Requirements

### Requirement: Build pipeline SHALL have three stages
The build pipeline SHALL execute as Parse → Enrich → Analyze → Persist, where each stage reads from and writes to the in-memory graph.

#### Scenario: Parse stage extracts code structure
- **WHEN** `kgraph build` runs
- **THEN** the Parse stage SHALL use existing language-specific extractors to produce nodes and edges

#### Scenario: Enrich stage adds semantic metadata
- **WHEN** Parse completes
- **THEN** the Enrich stage SHALL run rationale extraction and confidence annotation on the in-memory graph

#### Scenario: Analyze stage computes structural properties
- **WHEN** Enrich completes
- **THEN** the Analyze stage SHALL compute degree, detect god nodes, and detect communities

#### Scenario: Persist stage saves enriched graph
- **WHEN** Analyze completes
- **THEN** the Persist stage SHALL save the fully enriched graph to SQLite (existing SaveGraph logic)

### Requirement: Incremental update SHALL re-run enrich and analyze
When `kgraph update` re-extracts changed files, the enrich and analyze stages SHALL re-run on the affected portion of the graph.

#### Scenario: Update re-enriches changed files
- **WHEN** `kgraph update` re-extracts a changed file
- **THEN** the Enrich stage SHALL re-run on the newly extracted subgraph

#### Scenario: Update re-analyzes affected nodes
- **WHEN** edges change for any node during update
- **THEN** the Analyze stage SHALL recompute degree and god_node for affected nodes
- **THEN** communities SHALL be recomputed globally (since one edge change can affect community boundaries)

### Requirement: Build SHALL support --no-analyze flag
The user SHALL be able to skip the Analyze stage for faster builds that only need parsing and enrichment.

#### Scenario: Build without analytics
- **WHEN** the user runs `kgraph build --no-analyze`
- **THEN** the Parse and Enrich stages SHALL run
- **THEN** the Analyze stage SHALL be skipped
- **THEN** nodes SHALL NOT have community, god_node, or degree properties

### Requirement: Analyze SHALL be runnable independently
The user SHALL be able to run `kgraph analyze` to recompute analytics on an already-built graph without re-parsing.

#### Scenario: Analyze on existing graph
- **WHEN** the user runs `kgraph analyze`
- **THEN** the system SHALL load the existing graph from SQLite
- **THEN** the Analyze stage SHALL run (degree, god nodes, communities)
- **THEN** the enriched graph SHALL be saved back to SQLite

#### Scenario: Analyze with reclustering
- **WHEN** the user runs `kgraph analyze --recluster`
- **THEN** communities SHALL be recomputed from scratch (ignoring existing community assignments)
