## ADDED Requirements

### Requirement: Every node SHALL have a computed degree
The degree of each node (count of incoming + outgoing edges) SHALL be computed and stored as a node property after build.

#### Scenario: Degree computed after build
- **WHEN** `kgraph build` completes
- **THEN** every node in the graph SHALL have Properties["degree"] set to the count of its in-edges + out-edges

#### Scenario: Degree updated on incremental update
- **WHEN** `kgraph update` modifies edges for a node
- **THEN** the affected nodes' degrees SHALL be recomputed

### Requirement: God nodes SHALL be detected and marked
The top N nodes by degree (default N=10, configurable) SHALL be marked as god nodes.

#### Scenario: God nodes marked after build
- **WHEN** `kgraph build` completes on a repository with 500 nodes
- **THEN** the top 10 nodes by degree SHALL have Properties["god_node"] = true
- **THEN** all other nodes SHALL have Properties["god_node"] = false or absent

#### Scenario: God node count is configurable
- **WHEN** the user runs `kgraph build --god-nodes 20`
- **THEN** the top 20 nodes by degree SHALL be marked as god nodes

### Requirement: Communities SHALL be detected via clustering
The graph SHALL be partitioned into communities using a heuristic clustering algorithm. Each node SHALL be assigned a community ID.

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

### Requirement: Analytics SHALL be exposed via API
The server API SHALL expose god nodes, communities, and analytics metadata.

#### Scenario: God nodes endpoint
- **WHEN** a client requests GET /api/analytics/god-nodes
- **THEN** the response SHALL contain a JSON array of god-node objects with id, type, degree, and file

#### Scenario: Communities endpoint
- **WHEN** a client requests GET /api/analytics/communities
- **THEN** the response SHALL contain a JSON array of community objects with id, label, and node count

#### Scenario: Node detail includes analytics
- **WHEN** a client requests GET /api/node?id=X
- **THEN** the response SHALL include degree, god_node, community, and community_label fields

### Requirement: Graph report SHALL summarize analytics
`kgraph report` SHALL generate a GRAPH_REPORT.md summarizing god nodes, communities, surprising cross-community connections, and suggested questions.

#### Scenario: Report generation
- **WHEN** the user runs `kgraph report`
- **THEN** a GRAPH_REPORT.md file SHALL be generated containing: god nodes ranked by degree, community descriptions, top cross-community edges ranked by surprise score, and 4-5 suggested questions derived from graph structure

#### Scenario: Report includes surprising connections
- **WHEN** two nodes in different communities have an edge between them
- **THEN** that edge SHALL be listed as a "surprising connection" in the report, ranked by how unexpected it is (edges between communities with few inter-connections rank higher)
