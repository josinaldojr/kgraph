## MODIFIED Requirements

### Requirement: Communities SHALL be detected via clustering
The graph SHALL be partitioned into communities using modularity-optimization clustering (Louvain: iterative local node moves that maximize modularity, followed by graph aggregation, repeated until no further gain). Each node SHALL be assigned a community ID. The `resolution` parameter SHALL map to Louvain's own resolution parameter: higher resolution favors more, smaller communities; lower resolution favors fewer, larger ones.

#### Scenario: Communities assigned after build
- **WHEN** `kgraph build` completes
- **THEN** every node SHALL have Properties["community"] set to an integer community ID
- **THEN** nodes in the same tightly-connected subgraph SHALL share the same community ID

#### Scenario: Community labels generated
- **WHEN** communities are detected
- **THEN** each community SHALL have a label derived from the most common node types or names within it (e.g., "Auth", "Data Access", "HTTP")
- **THEN** each node SHALL have Properties["community_label"] set to its community's label

#### Scenario: Reclustering without rebuild
- **WHEN** the user runs `kgraph analyze --recluster --resolution 1.5`
- **THEN** communities SHALL be recomputed with the given resolution parameter without re-parsing source code

#### Scenario: Deterministic output for a fixed input
- **WHEN** community detection runs twice on the same graph with the same resolution, without any edit to the graph in between
- **THEN** both runs SHALL produce the same community assignment for every node — node evaluation order and modularity-gain tie-breaking SHALL NOT depend on map iteration order or any other non-deterministic source
