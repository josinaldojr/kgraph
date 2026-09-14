## MODIFIED Requirements

### Requirement: Server API SHALL expose query endpoints
The HTTP server SHALL expose new endpoints for query, path, and explain operations in addition to existing graph, graph/local, node, and search endpoints.

#### Scenario: Query endpoint
- **WHEN** a client sends GET /api/query?q=how+does+auth+work&maxTokens=5000
- **THEN** the server SHALL return a JSON response with the query subgraph (nodes, edges, relevance scores)

#### Scenario: Path endpoint
- **WHEN** a client sends GET /api/path?src=auth.AuthService&dst=db.DatabasePool
- **THEN** the server SHALL return a JSON response with the shortest path as an array of hops

#### Scenario: Explain endpoint
- **WHEN** a client sends GET /api/explain?id=auth.AuthService
- **THEN** the server SHALL return a JSON response with full node details including summary, rationale, relations, community, god_node, and degree

### Requirement: Node detail API SHALL include analytics fields
The existing GET /api/node endpoint SHALL include analytics metadata in its response.

#### Scenario: Node detail includes degree and community
- **WHEN** a client requests GET /api/node?id=X
- **THEN** the response SHALL include degree (int), god_node (bool), community (int), and community_label (string) fields

### Requirement: Graph API SHALL include edge confidence
The existing GET /api/graph and GET /api/graph/local endpoints SHALL include edge confidence in their responses.

#### Scenario: Graph response includes confidence
- **WHEN** a client requests GET /api/graph
- **THEN** each edge in the response SHALL include a confidence field ("EXTRACTED" or "INFERRED")

### Requirement: Hub mode SHALL support all new endpoints
All new endpoints SHALL be available in hub mode under /api/projects/{key}/ prefix.

#### Scenario: Hub query endpoint
- **WHEN** a client sends GET /api/projects/{key}/query?q=auth
- **THEN** the server SHALL return the query result for that specific project's graph
