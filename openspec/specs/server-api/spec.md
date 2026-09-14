# server-api

## Purpose

TBD - extracted from kgraph-graphify-evolution change. Defines new HTTP API endpoints (query, path, explain), analytics and confidence fields added to existing endpoints, and parity of all new endpoints under hub mode's per-project routes.

## Requirements

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

### Requirement: Query-parameter-driven work SHALL be bounded
The HTTP server SHALL cap `topK` (`/api/search`), `hops` (`/api/graph/local`, `/api/query`, `/api/explain`), and `max_tokens` (`/api/query`) at fixed maximums, clamping any requested value above the maximum down to it rather than performing unbounded work or erroring.

#### Scenario: Oversized hops clamps instead of erroring
- **WHEN** a client sends GET /api/query?q=auth&hops=999999
- **THEN** the server SHALL treat the request as if `hops` were the maximum allowed value, not the literal requested value, and SHALL return a normal JSON response (not an error)

#### Scenario: Oversized topK clamps instead of erroring
- **WHEN** a client sends GET /api/search?q=auth&topK=999999
- **THEN** the server SHALL return at most the maximum allowed number of results

#### Scenario: Oversized max_tokens clamps instead of erroring
- **WHEN** a client sends GET /api/query?q=auth&max_tokens=999999999
- **THEN** the server SHALL render the response within the maximum allowed token budget, not the literal requested value

### Requirement: A handler panic SHALL NOT crash the server process
The HTTP server SHALL recover from a panic in any request handler, log it, and return an HTTP 500 response for that request, leaving the process — and every other project served by it in hub mode — running.

#### Scenario: Panicking handler isolated to one request
- **WHEN** a request causes a handler to panic
- **THEN** the server SHALL respond to that request with HTTP 500
- **THEN** the server process SHALL remain running and continue serving subsequent requests, including for other projects under hub mode
