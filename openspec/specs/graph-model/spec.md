# graph-model

## Purpose

TBD - extracted from kgraph-graphify-evolution change. Defines extensions to the core graph data model: analytics metadata carried in node Properties with typed getters, the edge Confidence field, analytics query methods on Graph, and the new Rationale node type with its `explains` edge and deterministic ID scheme.

## Requirements

### Requirement: Node Properties SHALL be a flexible metadata bag
Node Properties map[string]any SHALL hold both code-model metadata (name, package, language) and analytics metadata (degree, community, god_node, rationale_count). Typed getters SHALL provide convenient access.

#### Scenario: Properties contain analytics metadata
- **WHEN** a graph is built with analytics enabled
- **THEN** each node's Properties map SHALL contain "degree" (int), "community" (int), "community_label" (string), and "god_node" (bool) keys

#### Scenario: Typed getters available
- **WHEN** code needs to read analytics metadata
- **THEN** typed getter methods (Node.Community() int, Node.IsGodNode() bool, Node.Degree() int) SHALL be available on the Node struct

### Requirement: Edge SHALL have a Confidence field
The Edge struct SHALL have a `Confidence string` field in addition to the existing ID, Type, SrcID, DstID, and Properties fields.

#### Scenario: Edge struct includes Confidence
- **WHEN** an Edge is created
- **THEN** it SHALL have a Confidence field that accepts "EXTRACTED" or "INFERRED"

#### Scenario: Confidence serialized to JSON
- **WHEN** an Edge is serialized to JSON (for API responses or export)
- **THEN** the Confidence field SHALL be included as a top-level field (not inside Properties)

### Requirement: Graph SHALL support analytics queries
The Graph struct SHALL provide methods for querying analytics data: GodNodes(), Communities(), NodesByCommunity(communityID).

#### Scenario: GodNodes returns top-N nodes
- **WHEN** g.GodNodes(10) is called
- **THEN** it SHALL return the 10 nodes with highest degree

#### Scenario: NodesByCommunity filters by community
- **WHEN** g.NodesByCommunity(2) is called
- **THEN** it SHALL return only nodes with Properties["community"] == 2

### Requirement: Rationale SHALL be a node type
A new NodeType `Rationale` SHALL be added, with Properties containing "kind" (NOTE/WHY/HACK/DOCSTRING/TODO/FIXME/WARNING) and "text" (the comment content).

#### Scenario: Rationale node type registered
- **WHEN** the graph model is loaded
- **THEN** NodeType "Rationale" SHALL be a valid node type

#### Scenario: Rationale has explains edge type
- **WHEN** a Rationale node is linked to a code entity
- **THEN** the edge type SHALL be "explains" (new EdgeType)

### Requirement: Node ID for Rationale SHALL be deterministic
Rationale node IDs SHALL follow the format `rationale:<file>:<line>:<kind>` to ensure idempotent rebuilds.

#### Scenario: Same rationale produces same ID
- **WHEN** a NOTE comment at line 42 of auth.go is extracted twice (e.g., rebuild)
- **THEN** the node ID SHALL be `rationale:auth.go:42:NOTE` both times

### Requirement: NodeType SHALL cover multi-language constructs
In addition to the Go-native node types, the graph model SHALL define NodeTypes `Enum`, `Decorator`, `Variable`, and `TypeAlias` to represent constructs common to Java, TypeScript, JavaScript, and Python that have no direct Go equivalent.

#### Scenario: Enum type registered
- **WHEN** a Java, TypeScript, or Python extractor encounters an enumerated type
- **THEN** it SHALL create a node with NodeType `Enum`

#### Scenario: Decorator type registered
- **WHEN** a Java annotation, TypeScript/JavaScript decorator, or Python decorator is extracted as an entity in its own right (as opposed to a property on another node)
- **THEN** it SHALL create a node with NodeType `Decorator`

#### Scenario: Variable type registered
- **WHEN** a JavaScript or Python module-level variable is extracted
- **THEN** it SHALL create a node with NodeType `Variable`

#### Scenario: TypeAlias type registered
- **WHEN** a TypeScript `type X = ...` alias is extracted
- **THEN** it SHALL create a node with NodeType `TypeAlias`

### Requirement: EdgeType SHALL cover multi-language relationships
In addition to the Go-native edge types, the graph model SHALL define EdgeTypes `extends`, `injected`, `decorated`, and `routed` to represent relationships common to framework-oriented, non-Go languages.

#### Scenario: Class/interface inheritance
- **WHEN** a Java, TypeScript, or Python class or interface extends another
- **THEN** an edge of type `extends` SHALL be created from the child node to the parent node

#### Scenario: Dependency injection
- **WHEN** a consumer is wired to a provider via a DI mechanism (e.g. Java `@Autowired`, TypeScript `@Inject`)
- **THEN** an edge of type `injected` SHALL be created from the consumer node to the provider node

#### Scenario: Decorator/annotation application
- **WHEN** an annotation or decorator is applied to an entity (class, method, field)
- **THEN** an edge of type `decorated` SHALL be created from the entity node to the `Decorator` node

#### Scenario: HTTP route mapping
- **WHEN** a handler is mapped to an HTTP route (e.g. Java `@GetMapping`, TypeScript `@Get`, Python `@app.route`)
- **THEN** an edge of type `routed` SHALL be created from the handler node to the `Endpoint` node

### Requirement: Multi-language node types SHALL have discriminated, deterministic IDs
Because Java, TypeScript, JavaScript, and Python do not guarantee unique top-level identifiers per package/module the way Go does (e.g. a Python module-level variable can share a name with a function), the ID generators for `Enum`, `Decorator`, `Variable`, and `TypeAlias` SHALL include a type discriminator prefix so they cannot collide with each other or with a same-named `Struct`/`Interface` node.

#### Scenario: EnumID is discriminated and deterministic
- **WHEN** `EnumID(importPath, name)` is called
- **THEN** it SHALL return `enum:<importPath>.<name>`, and calling it again with the same arguments SHALL return the same ID

#### Scenario: DecoratorID is discriminated and deterministic
- **WHEN** `DecoratorID(importPath, name)` is called
- **THEN** it SHALL return `decorator:<importPath>.<name>`, and calling it again with the same arguments SHALL return the same ID

#### Scenario: VariableID is discriminated and deterministic
- **WHEN** `VariableID(importPath, name)` is called
- **THEN** it SHALL return `variable:<importPath>.<name>`, and calling it again with the same arguments SHALL return the same ID

#### Scenario: TypeAliasID is discriminated and deterministic
- **WHEN** `TypeAliasID(importPath, name)` is called
- **THEN** it SHALL return `typealias:<importPath>.<name>`, and calling it again with the same arguments SHALL return the same ID

#### Scenario: Existing IDs unaffected
- **WHEN** the multi-language NodeTypes and ID generators are added
- **THEN** no existing NodeType, EdgeType, or ID generator's output changes, and the SQLite schema (NodeType/EdgeType stored as free-form strings) requires no migration
