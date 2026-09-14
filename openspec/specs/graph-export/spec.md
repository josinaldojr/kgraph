# graph-export

## Purpose

TBD - extracted from kgraph-graphify-evolution change. Defines `kgraph export` (a portable graph.json artifact containing the full enriched graph) and `kgraph report` (a human-readable GRAPH_REPORT.md summary), including output path configuration.

## Requirements

### Requirement: graph.json SHALL export the complete enriched graph
`kgraph export` SHALL produce a graph.json file containing all nodes (with all Properties including analytics metadata), all edges (with confidence), summaries, community metadata, and build metadata.

#### Scenario: Full graph export
- **WHEN** the user runs `kgraph export`
- **THEN** a graph.json file SHALL be created containing: nodes array (each with id, type, file, signature, summary, community, community_label, degree, god_node, rationale), edges array (each with type, src, dst, confidence), communities array (each with id, label, node_ids), god_nodes array, and metadata object (repo_path, built_at, node_count, edge_count, languages)

#### Scenario: Export is portable
- **WHEN** graph.json is produced
- **THEN** it SHALL be valid JSON consumable by any tool without kgraph-specific dependencies
- **THEN** all node IDs SHALL be deterministic strings (not auto-incremented integers)

#### Scenario: Export includes rationale
- **WHEN** the graph contains Rationale nodes
- **THEN** each source node in the export SHALL include a `rationale` array with kind and text from linked Rationale nodes

### Requirement: GRAPH_REPORT.md SHALL summarize the codebase
`kgraph report` SHALL generate a GRAPH_REPORT.md containing: god nodes, community descriptions, surprising connections, and suggested questions.

#### Scenario: Report contains god nodes section
- **WHEN** `kgraph report` is run
- **THEN** the report SHALL contain a "God Nodes" section listing the top-N most connected nodes with their degree, type, and file

#### Scenario: Report contains communities section
- **WHEN** `kgraph report` is run
- **THEN** the report SHALL contain a "Communities" section listing each community with its label, node count, and representative nodes

#### Scenario: Report contains surprising connections
- **WHEN** `kgraph report` is run
- **THEN** the report SHALL contain a "Surprising Connections" section listing edges between different communities, ranked by surprise score (how isolated the two communities are from each other)

#### Scenario: Report contains suggested questions
- **WHEN** `kgraph report` is run
- **THEN** the report SHALL contain 4-5 "Suggested Questions" derived from graph structure (e.g., questions about god nodes, cross-community connections, or high-degree nodes)

### Requirement: Export SHALL support output path configuration
The user SHALL be able to specify the output directory for export files.

#### Scenario: Custom output directory
- **WHEN** the user runs `kgraph export --output /tmp/graph-out`
- **THEN** graph.json and GRAPH_REPORT.md SHALL be written to /tmp/graph-out/

#### Scenario: Default output directory
- **WHEN** the user runs `kgraph export` without --output
- **THEN** files SHALL be written to ./kgraph-out/ directory
