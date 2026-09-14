## ADDED Requirements

### Requirement: Maven/Gradle dependencies SHALL be connected to the graph via imports edges
Each `ExternalDependency` node produced from a Maven or Gradle dependency declaration SHALL be reachable from at least one `Package` node: when a Java source file's `import` statement resolves to an external (non-repo-internal) type, the extractor SHALL create an `imports` edge from the importing file's `Package` node to the corresponding `ExternalDependency` node, creating that node on the fly (tagged `Properties["language"] = "java"`) if no manifest-derived node already exists for it.

#### Scenario: Import of an external type creates an imports edge to its dependency node
- **WHEN** a Java file contains `import org.springframework.stereotype.Service;` and `org.springframework` is not a package produced by this repository's own source
- **THEN** an `imports` edge is created from that file's `Package` node to an `ExternalDependency` node representing `org.springframework`

#### Scenario: Internal import does not create an ExternalDependency edge
- **WHEN** a Java file imports a type from a package also produced by this repository's own source
- **THEN** the `imports` edge targets that internal `Package` node, not an `ExternalDependency` node
