## ADDED Requirements

### Requirement: Rationale comments SHALL be extracted as nodes
Comments in source code tagged with recognized rationale prefixes (NOTE:, WHY:, HACK:, TODO:, FIXME:, WARNING:) SHALL be extracted as first-class Rationale nodes in the graph.

#### Scenario: NOTE comment in Go source
- **WHEN** a Go file contains a comment `// NOTE: Uses stateless JWT for horizontal scaling`
- **THEN** a Rationale node SHALL be created with Properties containing kind="NOTE" and text="Uses stateless JWT for horizontal scaling"
- **THEN** an `explains` edge SHALL connect the Rationale node to the nearest code entity (function, struct, or package)

#### Scenario: WHY comment in Python source
- **WHEN** a Python file contains `# WHY: This avoids N+1 queries on the hot path`
- **THEN** a Rationale node SHALL be created with kind="WHY" and the comment text
- **THEN** an `explains` edge SHALL connect it to the enclosing function or class

#### Scenario: HACK comment in TypeScript source
- **WHEN** a TypeScript file contains `// HACK: Working around upstream API bug, remove after v3`
- **THEN** a Rationale node SHALL be created with kind="HACK" and the comment text

### Requirement: Docstrings SHALL be extracted as rationale nodes
Module-level, class-level, and function-level docstrings SHALL be extracted as Rationale nodes with kind="DOCSTRING", linked via `explains` edges to the entity they document.

#### Scenario: Go doc comment
- **WHEN** a Go function has a doc comment above it (the standard `// FuncName ...` pattern)
- **THEN** a Rationale node SHALL be created with kind="DOCSTRING" and the doc text
- **THEN** an `explains` edge SHALL connect it to the Function node

#### Scenario: Python docstring
- **WHEN** a Python function or class has a triple-quoted docstring as its first statement
- **THEN** a Rationale node SHALL be created with kind="DOCSTRING" and the doc text

### Requirement: Rationale nodes SHALL have deterministic IDs
Rationale node IDs SHALL be derived from the file path, line number, and kind, ensuring idempotent rebuilds.

#### Scenario: Same rationale on rebuild
- **WHEN** the same comment exists at the same file and line after a rebuild
- **THEN** the Rationale node ID SHALL be identical, preventing duplicates

### Requirement: Rationale SHALL appear in context and explain output
When generating context for a node, any Rationale nodes linked via `explains` edges SHALL be included in the output.

#### Scenario: Context includes rationale
- **WHEN** `kgraph context SomeFunction` is run and SomeFunction has 2 rationale nodes linked via `explains`
- **THEN** the context output SHALL include a "Rationale" section with both rationale texts

#### Scenario: Explain includes rationale
- **WHEN** `kgraph explain SomeFunction` is run
- **THEN** the explain output SHALL include all rationale nodes connected via `explains` edges, showing kind and text
