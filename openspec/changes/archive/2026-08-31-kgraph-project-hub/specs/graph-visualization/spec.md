## ADDED Requirements

### Requirement: Project selection on hub startup
The system SHALL, when `kgraph serve` runs in hub mode, present at the root URL a project-selection view instead of a graph: one entry per discovered built project showing its display name, recorded repository path, node and edge counts, last build time, and freshness status. A graph SHALL only be displayed after an explicit project choice. Projects whose repository path no longer exists SHALL be listed with that condition visibly flagged but remain selectable (their graph is still viewable as the last built snapshot); projects whose database is unreadable SHALL be listed with that condition and SHALL NOT be selectable. When no built graphs exist, the view SHALL say so and point at `kgraph build`.

#### Scenario: Hub root shows the picker, not a graph
- **WHEN** a person opens the root URL of a hub-mode `kgraph serve`
- **THEN** the system shows the project-selection view with every discovered project, and does not render any graph canvas for a specific project

#### Scenario: Choosing a project opens its viewer
- **WHEN** a person selects a project from the selection view
- **THEN** the system opens that project's graph viewer with its own graph data

#### Scenario: Orphaned project remains viewable but flagged
- **WHEN** the selection view contains a project whose recorded repository path no longer exists
- **THEN** that project is visibly marked as repository missing, and selecting it still opens its last built graph snapshot

#### Scenario: Empty hub points at building
- **WHEN** a person opens the root URL of a hub-mode `kgraph serve` and no graph databases have been discovered
- **THEN** the view states that no graphs have been built yet and indicates that `kgraph build` creates them

### Requirement: Per-project viewer routes with lazy loading
The system SHALL serve each project's viewer under a distinct route keyed by that project's cache-directory name (e.g. `/p/<key>/`), so a project view can be linked to directly, and SHALL load a project's graph on demand at first visit rather than loading every discovered graph when the server starts. All existing viewer endpoints (graph, local graph, node detail, search, live-refresh stream) SHALL be available scoped per project.

#### Scenario: Direct link opens the right project
- **WHEN** a person opens a project's viewer route directly (e.g. a bookmarked or shared URL)
- **THEN** the system displays that project's graph viewer without requiring a prior visit to the selection view

#### Scenario: Graphs load on demand
- **WHEN** the hub server starts and no project route has been visited yet
- **THEN** no project graph is loaded into memory, and visiting one project's route loads only that project's graph

#### Scenario: Unknown project key is rejected
- **WHEN** a request targets a project route whose key matches no discovered database
- **THEN** the system responds with a clear not-found error rather than an empty or misleading graph

### Requirement: Per-project live refresh isolation
The system SHALL scope live-refresh events to the project whose graph changed: a rebuild or update of one project's database SHALL refresh only views of that project, and SHALL NOT cause re-renders, flicker, or refresh events in views of any other project.

#### Scenario: Rebuild refreshes only its own project view
- **WHEN** a person is viewing project A's graph and `kgraph update` finishes against project B's database
- **THEN** project A's view does not re-render or emit a refresh event, while any open view of project B reflects the update

#### Scenario: Update to the viewed project still refreshes live
- **WHEN** a person is viewing a project's graph and `kgraph build` or `kgraph update` finishes against that same project's database
- **THEN** the open view reflects the new graph without a manual page reload, per the existing live-refresh requirement

### Requirement: Single-project mode keeps the unprefixed viewer
The system SHALL, when `kgraph serve` runs in single-project mode (`--repo` or `--pick`), keep serving the viewer at the root URL with the unprefixed API surface, with no project-selection view, preserving the pre-hub behavior.

#### Scenario: Legacy URL keeps working
- **WHEN** a person opens the root URL of a single-project `kgraph serve` started with `--repo`
- **THEN** the system renders that repository's graph viewer directly, exactly as before hub mode existed
