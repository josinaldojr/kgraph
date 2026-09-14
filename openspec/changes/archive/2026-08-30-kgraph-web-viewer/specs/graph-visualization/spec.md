## ADDED Requirements

### Requirement: Force-directed graph rendering
The system SHALL render the knowledge graph as an interactive force-directed node-link diagram — nodes as circles positioned by a physics simulation (mutual repulsion, edges as springs), edge lines connecting related nodes — in the style of Obsidian's graph view, rather than a hierarchical drill-down tree or a static layout.

#### Scenario: Global graph on load
- **WHEN** a person opens the viewer with no node selected
- **THEN** the system renders every visible (per active type filters) node and edge as an animated force-directed layout

#### Scenario: Node visual weight reflects connectivity
- **WHEN** the global graph is rendered
- **THEN** a node's circle radius scales with its number of edges, and its color encodes its node type, consistently across the view

#### Scenario: Dragging a node repositions it
- **WHEN** a person drags a node to a new position
- **THEN** the node moves to that position and the simulation settles the rest of the graph around it, rather than snapping back

### Requirement: Type-based filtering
The system SHALL let a person show or hide nodes by `NodeType` (e.g. hide `Field`/`Column`, show `Table`/`Endpoint`/`ExternalDependency`), with `Field` and `Column` hidden by default.

#### Scenario: Hiding a node type
- **WHEN** a person toggles off a node type in the filter controls
- **THEN** nodes of that type (and edges solely connecting to them) disappear from the current graph view without a full page reload

#### Scenario: Default filters keep the initial view uncluttered
- **WHEN** a person opens the viewer for the first time
- **THEN** `Field` and `Column` nodes are hidden by default, while all other node types are visible

### Requirement: Hover and selection highlighting
The system SHALL, on hovering a node, visually emphasize that node and its directly connected neighbors while dimming all other nodes and edges, matching Obsidian's graph-view hover behavior.

#### Scenario: Hovering a node in the global graph
- **WHEN** a person hovers over a node
- **THEN** that node, its direct neighbors, and the edges between them render at full opacity while all other nodes and edges dim

#### Scenario: Hover ends
- **WHEN** the pointer moves away from the hovered node
- **THEN** the graph returns to its normal (undimmed) rendering

### Requirement: Local graph view centered on a selected node
The system SHALL provide a local graph mode that, given a selected node and a hop count, renders only that node's subgraph out to the given number of hops — reusing the same hop-based subgraph expansion the `context` command already performs — with a visible way to return to the global graph.

#### Scenario: Selecting a node opens its local graph
- **WHEN** a person clicks a node in the global graph or opens a node from search results
- **THEN** the view switches to a local graph centered on that node, showing only nodes and edges within the default hop distance

#### Scenario: Returning to the global graph
- **WHEN** a person is in local graph mode and chooses to return to the global view
- **THEN** the view switches back to rendering the full (filtered) graph

### Requirement: Zoom and pan
The system SHALL support zooming in/out (e.g. via scroll) and panning (e.g. via drag on empty canvas space) over the rendered graph, in both global and local modes.

#### Scenario: Zooming into a dense cluster
- **WHEN** a person scrolls to zoom in over a cluster of nodes
- **THEN** the view magnifies around that point without breaking the running force simulation

### Requirement: Node detail view
The system SHALL show, for any selected node, its note (summary) if one exists, its signature, its file and line range when applicable, and its outgoing and incoming edges with their types and the nodes they connect to.

#### Scenario: Selecting a summarized node
- **WHEN** a person selects a node that has a current-hash summary
- **THEN** the detail view displays that summary text alongside the node's signature, file/line, and its in/out edges

#### Scenario: Selecting a node with no note yet
- **WHEN** a person selects a node that has never been summarized
- **THEN** the detail view indicates no note exists yet, rather than showing a blank or erroring

#### Scenario: Selecting a node with a stale note
- **WHEN** a person selects a node whose stored summary is marked stale
- **THEN** the detail view displays the existing summary text but visibly marks it as stale/out of date

### Requirement: Search-to-focus
The system SHALL provide a search box that queries the graph using the existing lexical `SearchNodes` ranking and, on selecting a result, switches to local graph mode centered on that node and opens its detail view.

#### Scenario: Searching by topic
- **WHEN** a person types a natural-language query into the search box
- **THEN** the results are ordered by descending match score, matching `SearchNodes`' own ranking, and selecting a result switches to local graph mode centered on it and opens its detail view

### Requirement: Live refresh on graph change
The system SHALL detect when the underlying graph has been rebuilt or updated by a separate `kgraph build`/`kgraph update` process and refresh connected browser views without requiring a manual page reload or a server restart.

#### Scenario: Update lands while the viewer is open
- **WHEN** `kgraph update` finishes running against the same database `kgraph serve` has open, adding or changing nodes
- **THEN** a browser with the viewer already open reflects the change without the person needing to reload the page

#### Scenario: No change, no refresh churn
- **WHEN** no `build`/`update` has run since the last refresh
- **THEN** the viewer does not re-render or flicker due to the change-detection mechanism

### Requirement: Read-only against the graph store
The system SHALL only read from the graph database, and SHALL NOT create, modify, or delete any node, edge, summary, or build-metadata row, regardless of what a person does in the browser.

#### Scenario: Viewer runs alongside a concurrent build
- **WHEN** `kgraph serve` is running against a database while a separate `kgraph build` or `kgraph update` process writes to the same database file
- **THEN** the viewer continues to serve reads without corrupting or blocking the concurrent write, and performs no writes of its own

### Requirement: Local-only binding by default
The system SHALL bind its HTTP listener to localhost by default, requiring an explicit operator choice to bind elsewhere.

#### Scenario: Default startup
- **WHEN** `kgraph serve` is started without an explicit bind address
- **THEN** the server is reachable from the local machine only, not from other hosts on the network
