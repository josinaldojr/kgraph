## ADDED Requirements

### Requirement: requirements.txt/pyproject.toml dependencies SHALL be connected to the graph via imports edges
Each `ExternalDependency` node produced from a `requirements.txt` or `pyproject.toml` dependency entry SHALL be reachable from at least one `Package` node: when a Python `import`/`from ... import` statement resolves to an external (non-repo-internal) module, the extractor SHALL create an `imports` edge from the importing module's `Package` node to the corresponding `ExternalDependency` node, creating that node on the fly (tagged `Properties["language"] = "python"`) if no manifest-derived node already exists for it, instead of creating a bare `Package` node for the external target.

#### Scenario: Import of an external module creates an imports edge to its dependency node
- **WHEN** a Python file contains `import flask` and `flask` is not a module produced by this repository's own source
- **THEN** an `imports` edge is created from that file's `Package` node to an `ExternalDependency` node representing `flask`, and no bare, property-less `Package` node is created for `flask`

#### Scenario: Internal import does not create an ExternalDependency edge
- **WHEN** a Python file imports a module also produced by this repository's own source
- **THEN** the `imports` edge targets that internal `Package` node, not an `ExternalDependency` node
