# query-engine

## Purpose

TBD - extracted from kgraph-graphify-evolution change. Defines the natural-language query, path-finding, explain, and prompt-generation CLI commands that build on the existing context-assembly engine (BFS expansion, token estimation, rendering) to answer questions about the graph.

## Requirements

### Requirement: Query command SHALL answer natural-language questions
`kgraph query "<question>"` SHALL return a subgraph of the most relevant nodes and edges for the given question, using lexical relevance scoring and community-aware scoping.

#### Scenario: Query returns relevant subgraph
- **WHEN** the user runs `kgraph query "how does authentication work"`
- **THEN** the system SHALL search for nodes matching authentication-related terms
- **THEN** the system SHALL expand the BFS subgraph from matching nodes
- **THEN** the output SHALL include the most relevant nodes (ranked by relevance score) within the token budget

#### Scenario: Query respects token budget
- **WHEN** the user runs `kgraph query "database connections" --max-tokens 5000`
- **THEN** the output SHALL not exceed approximately 5000 tokens
- **THEN** the most relevant nodes SHALL appear first (no truncation of high-relevance nodes)

#### Scenario: Query scopes by community
- **WHEN** the query matches nodes in a specific community
- **THEN** other nodes from the same community SHALL be boosted in relevance

### Requirement: Path command SHALL find shortest path between two nodes
`kgraph path <source> <target>` SHALL compute and display the shortest weighted path between two nodes.

#### Scenario: Path found between two functions
- **WHEN** the user runs `kgraph path AuthService.Login DatabasePool.Query`
- **THEN** the system SHALL compute the shortest path using edge weights (calls=3, structural=2, imports=1)
- **THEN** the output SHALL display each hop with the edge type connecting them

#### Scenario: No path exists
- **WHEN** the user runs `kgraph path A B` and no path exists between A and B
- **THEN** the system SHALL report "no path found" without error

#### Scenario: Path between nodes in different communities
- **WHEN** source and target are in different communities
- **THEN** the path SHALL still be found (communities don't restrict traversal)

### Requirement: Explain command SHALL provide rich node context
`kgraph explain <node>` SHALL output comprehensive information about a node: signature, summary, rationale, relations, community, and god-node status.

#### Scenario: Explain a struct node
- **WHEN** the user runs `kgraph explain AuthService`
- **THEN** the output SHALL contain: node type, file location, signature, summary (if available), rationale nodes linked via `explains`, grouped relations (by edge type and direction), community label, god-node flag, and degree

#### Scenario: Explain includes confidence on relations
- **WHEN** the explain output lists relations
- **THEN** each relation SHALL show the confidence tag (EXTRACTED/INFERRED) of the connecting edge

### Requirement: Prompt command SHALL produce clipboard-ready LLM context
`kgraph prompt <node>` SHALL output the explain result formatted as a structured prompt suitable for pasting into an LLM conversation.

#### Scenario: Prompt formatted for LLM
- **WHEN** the user runs `kgraph prompt AuthService`
- **THEN** the output SHALL contain markdown sections: "## Context: AuthService", "### Direct Relations", "### Related Nodes", "### Rationale", "### Community"
- **THEN** the output SHALL be suitable for pasting as-is into an LLM prompt

#### Scenario: Prompt respects token budget
- **WHEN** the user runs `kgraph prompt AuthService --max-tokens 4000`
- **THEN** the output SHALL not exceed approximately 4000 tokens

### Requirement: Query commands SHALL reuse existing context engine
All query commands (query, path, explain, prompt) SHALL reuse the BFS expansion, token estimation, and rendering logic from `internal/context`.

#### Scenario: Query uses same BFS as context
- **WHEN** `kgraph query` expands a subgraph
- **THEN** it SHALL use the same `expandSubgraph` function as `kgraph context`

#### Scenario: Prompt uses same token estimation as context
- **WHEN** `kgraph prompt` truncates output
- **THEN** it SHALL use the same `EstimateTokens` function as `kgraph context`
