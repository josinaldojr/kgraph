## MODIFIED Requirements

### Requirement: Export SHALL support output path configuration
The user SHALL be able to specify the output directory for export files. `kgraph export` SHALL write three artifacts into that directory: `graph.json`, `GRAPH_REPORT.md`, and `graph.html` (the self-contained interactive viewer defined by `graph-html-export`).

#### Scenario: Custom output directory
- **WHEN** the user runs `kgraph export --output /tmp/graph-out`
- **THEN** graph.json, GRAPH_REPORT.md, and graph.html SHALL be written to /tmp/graph-out/

#### Scenario: Default output directory
- **WHEN** the user runs `kgraph export` without --output
- **THEN** graph.json, GRAPH_REPORT.md, and graph.html SHALL be written to ./kgraph-out/ directory
