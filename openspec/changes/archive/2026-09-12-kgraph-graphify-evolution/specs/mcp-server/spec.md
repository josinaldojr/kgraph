## ADDED Requirements

### Requirement: MCP server SHALL expose graph tools
An MCP server SHALL be available via `kgraph mcp` that exposes graph operations as MCP tools for AI coding assistants.

#### Scenario: MCP server starts via stdio
- **WHEN** the user runs `kgraph mcp --transport stdio`
- **THEN** an MCP server SHALL start on stdin/stdout, advertising available tools

#### Scenario: MCP server starts via HTTP
- **WHEN** the user runs `kgraph mcp --transport http --port 8080`
- **THEN** an MCP server SHALL listen on the specified port, accepting MCP tool calls over HTTP

### Requirement: MCP tool query_graph SHALL answer questions
The `query_graph` tool SHALL accept a natural-language question and return a scoped subgraph as JSON.

#### Scenario: Query via MCP
- **WHEN** an AI assistant calls `query_graph` with question="how does auth work"
- **THEN** the tool SHALL return a JSON subgraph with relevant nodes and edges, identical to `kgraph query` output

### Requirement: MCP tool get_node SHALL return node details
The `get_node` tool SHALL accept a node ID and return full node details including summary, rationale, relations, and analytics metadata.

#### Scenario: Get node via MCP
- **WHEN** an AI assistant calls `get_node` with id="auth.AuthService"
- **THEN** the tool SHALL return a JSON object with node details identical to `kgraph explain` output

### Requirement: MCP tool shortest_path SHALL trace connections
The `shortest_path` tool SHALL accept source and target node IDs and return the shortest path between them.

#### Scenario: Find path via MCP
- **WHEN** an AI assistant calls `shortest_path` with source="auth.AuthService" and target="db.DatabasePool"
- **THEN** the tool SHALL return a JSON array of hops, each with node_id, edge_type, and confidence

### Requirement: MCP tool export_graph SHALL provide the full graph
The `export_graph` tool SHALL return the complete graph as JSON (equivalent to graph.json export).

#### Scenario: Export via MCP
- **WHEN** an AI assistant calls `export_graph`
- **THEN** the tool SHALL return the full graph JSON with all nodes, edges, communities, and metadata

### Requirement: MCP server SHALL support authentication for HTTP transport
When running in HTTP transport mode, the MCP server SHALL support optional API key authentication.

#### Scenario: HTTP with API key
- **WHEN** the user runs `kgraph mcp --transport http --api-key mysecret`
- **THEN** requests without a valid `Authorization: Bearer mysecret` header SHALL be rejected with 401

#### Scenario: HTTP without API key
- **WHEN** the user runs `kgraph mcp --transport http` without --api-key
- **THEN** all requests SHALL be accepted (no authentication required)
