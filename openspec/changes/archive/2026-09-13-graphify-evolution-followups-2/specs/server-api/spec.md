## ADDED Requirements

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
