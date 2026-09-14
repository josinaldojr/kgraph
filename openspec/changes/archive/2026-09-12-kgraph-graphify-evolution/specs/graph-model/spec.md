## MODIFIED Requirements

### Requirement: Node Properties SHALL be a flexible metadata bag
Node Properties map[string]any SHALL hold both code-model metadata (name, package, language) and analytics metadata (degree, community, god_node, rationale_count). Typed getters SHALL provide convenient access.

#### Scenario: Properties contain analytics metadata
- **WHEN** a graph is built with analytics enabled
- **THEN** each node's Properties map SHALL contain "degree" (int), "community" (int), "community_label" (string), and "god_node" (bool) keys

#### Scenario: Typed getters available
- **WHEN** code needs to read analytics metadata
- **THEN** typed getter methods (Node.Community() int, Node.IsGodNode() bool, Node.Degree() int) SHALL be available on the Node struct

### Requirement: Edge SHALL have a Confidence field
The Edge struct SHALL have a `Confidence string` field in addition to the existing ID, Type, SrcID, DstID, and Properties fields.

#### Scenario: Edge struct includes Confidence
- **WHEN** an Edge is created
- **THEN** it SHALL have a Confidence field that accepts "EXTRACTED" or "INFERRED"

#### Scenario: Confidence serialized to JSON
- **WHEN** an Edge is serialized to JSON (for API responses or export)
- **THEN** the Confidence field SHALL be included as a top-level field (not inside Properties)

### Requirement: Graph SHALL support analytics queries
The Graph struct SHALL provide methods for querying analytics data: GodNodes(), Communities(), NodesByCommunity(communityID).

#### Scenario: GodNodes returns top-N nodes
- **WHEN** g.GodNodes(10) is called
- **THEN** it SHALL return the 10 nodes with highest degree

#### Scenario: NodesByCommunity filters by community
- **WHEN** g.NodesByCommunity(2) is called
- **THEN** it SHALL return only nodes with Properties["community"] == 2

### Requirement: Rationale SHALL be a node type
A new NodeType `Rationale` SHALL be added, with Properties containing "kind" (NOTE/WHY/HACK/DOCSTRING/TODO/FIXME/WARNING) and "text" (the comment content).

#### Scenario: Rationale node type registered
- **WHEN** the graph model is loaded
- **THEN** NodeType "Rationale" SHALL be a valid node type

#### Scenario: Rationale has explains edge type
- **WHEN** a Rationale node is linked to a code entity
- **THEN** the edge type SHALL be "explains" (new EdgeType)

### Requirement: Node ID for Rationale SHALL be deterministic
Rationale node IDs SHALL follow the format `rationale:<file>:<line>:<kind>` to ensure idempotent rebuilds.

#### Scenario: Same rationale produces same ID
- **WHEN** a NOTE comment at line 42 of auth.go is extracted twice (e.g., rebuild)
- **THEN** the node ID SHALL be `rationale:auth.go:42:NOTE` both times
