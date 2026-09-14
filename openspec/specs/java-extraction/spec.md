# java-extraction

## Purpose

Extracts a code knowledge graph from Java repositories: classes, interfaces, enums, methods, fields, imports, inheritance, calls, Spring/JPA annotations, and Maven/Gradle dependencies. This extractor is regex-based and best-effort (not AST-based), so accuracy and completeness are lower-fidelity than the Go extractor.

## Requirements

### Requirement: Declaration extraction SHALL cover packages, types, methods, and fields
The Java extractor SHALL parse `.java` source files and produce nodes for packages (`Package`, ID `pkg:<dotted.package>`), classes and abstract classes (`Struct`, with property `abstract=true` for abstract classes), records (`Struct` with property `record=true`), interfaces (`Interface`), enums (`Enum`), methods (`Function`), static methods (`Function` with property `static=true`), constructors (`Function` with property `constructor=true`), fields (`Field`), and static fields (`Field` with property `static=true`).

#### Scenario: Class produces a Struct node
- **WHEN** the extractor encounters a Java class declaration
- **THEN** it creates a `Struct` node with ID `<package>.<ClassName>`

#### Scenario: Abstract class flagged
- **WHEN** the extractor encounters a class declared `abstract`
- **THEN** the resulting `Struct` node has `Properties["abstract"] = true`

#### Scenario: Interface produces an Interface node
- **WHEN** the extractor encounters a Java interface declaration
- **THEN** it creates an `Interface` node with ID `<package>.<InterfaceName>`

#### Scenario: Enum produces an Enum node
- **WHEN** the extractor encounters a Java enum declaration
- **THEN** it creates an `Enum` node with ID `<package>.<EnumName>`

#### Scenario: Record flagged
- **WHEN** the extractor encounters a Java 16+ record declaration
- **THEN** it creates a `Struct` node with `Properties["record"] = true`

#### Scenario: Constructor produces a constructor-flagged Function
- **WHEN** the extractor encounters a class constructor
- **THEN** it creates a `Function` node with ID `<Class>.<init>()` and `Properties["constructor"] = true`

#### Scenario: Static method flagged
- **WHEN** the extractor encounters a method declared `static`
- **THEN** the resulting `Function` node has `Properties["static"] = true`

#### Scenario: Static field flagged
- **WHEN** the extractor encounters a field declared `static`
- **THEN** the resulting `Field` node has `Properties["static"] = true`

### Requirement: Relationship extraction SHALL cover imports, inheritance, calls, and membership
The Java extractor SHALL create `imports` edges from `import` statements, `extends` edges from class/interface inheritance, `implements` edges from `class ... implements ...`, `calls` edges from resolvable method-call expressions, `has_field` edges from field declarations, and `has_method` edges linking methods to their declaring class.

#### Scenario: Import creates imports edge
- **WHEN** a source file contains `import com.example.foo.Bar;`
- **THEN** an `imports` edge is created from the importing package/type to `com.example.foo.Bar`

#### Scenario: Class extends creates extends edge
- **WHEN** a class declares `class Foo extends Bar`
- **THEN** an `extends` edge is created from `Foo` to `Bar`

#### Scenario: Interface extends creates extends edge
- **WHEN** an interface declares `interface Foo extends Bar`
- **THEN** an `extends` edge is created from `Foo` to `Bar`

#### Scenario: Class implements creates implements edge
- **WHEN** a class declares `class Foo implements Bar`
- **THEN** an `implements` edge is created from `Foo` to `Bar`

#### Scenario: Method belongs to class
- **WHEN** a method is declared within a class body
- **THEN** a `has_method` edge is created from the class's `Struct` node to the method's `Function` node

### Requirement: Best-effort call resolution without full type-checking
The Java extractor SHALL resolve method calls on a best-effort basis without complete type inference: `this.method()` and unqualified `method()` resolve to a method on the same class; `variable.method()` resolves to a same-named method on an imported class; `StaticClass.method()` resolves to a static method. Calls that require polymorphic resolution via an interface SHALL be silently skipped rather than causing an error.

#### Scenario: This-call resolves within same class
- **WHEN** a method body contains `this.save()` or `save()`
- **THEN** a `calls` edge is created to the `save` method on the same class, if one exists

#### Scenario: Static call resolves to static method
- **WHEN** a method body contains `StaticClass.method()`
- **THEN** a `calls` edge is created to `StaticClass`'s static `method`, if resolvable

#### Scenario: Polymorphic interface call skipped
- **WHEN** a call is made through an interface reference and cannot be resolved to a concrete implementation
- **THEN** the extractor skips creating a `calls` edge for that call site without raising an error

### Requirement: Spring and JPA annotations SHALL be extracted as decorated/injected/routed/maps_to_table relationships
The Java extractor SHALL recognize `@Autowired`/`@Inject` as `injected` edges, `@Service`/`@Component`/`@Repository`/`@RestController`/`@Controller` as `decorated` edges, `@GetMapping`/`@PostMapping` and similar HTTP-method annotations as `routed` edges to `Endpoint` nodes, and `@Entity`/`@Table` as `maps_to_table` edges with `@Column` contributing column names via `has_field` edges.

#### Scenario: Autowired field creates injected edge
- **WHEN** a field is annotated `@Autowired` or `@Inject`
- **THEN** an `injected` edge is created from the declaring class to the field's declared type

#### Scenario: Service annotation creates decorated edge
- **WHEN** a class is annotated `@Service`, `@Component`, or `@Repository`
- **THEN** a `decorated` edge is created from the class to a `Decorator` node representing that annotation

#### Scenario: REST mapping annotation creates routed edge
- **WHEN** a method is annotated `@GetMapping`, `@PostMapping`, or a similar HTTP-mapping annotation, on a class annotated `@RestController` or `@Controller`
- **THEN** an `Endpoint` node is created for the combined path (class-level `@RequestMapping` base path plus method-level path) and a `routed` edge is created from the method to that `Endpoint` node

#### Scenario: Entity annotation creates maps_to_table edge
- **WHEN** a class is annotated `@Entity` (optionally with `@Table(name = "...")`)
- **THEN** a `Table` node is created (named per `@Table` if present, otherwise derived from the class name) and a `maps_to_table` edge is created from the class to the `Table` node

#### Scenario: Column annotation creates table column
- **WHEN** a field is annotated `@Column(name = "...")` within an `@Entity` class
- **THEN** a `Column` node is created under that entity's `Table` node, linked via `has_field`

### Requirement: Maven and Gradle dependencies SHALL be extracted as ExternalDependency nodes
The Java extractor SHALL parse `pom.xml` (Maven) and `build.gradle`/`build.gradle.kts` (Gradle) dependency declarations into `ExternalDependency` nodes carrying `group_id` and `artifact_id` properties (or their Gradle equivalents).

#### Scenario: Maven dependency extracted
- **WHEN** `pom.xml` declares a `<dependency>` with `<groupId>` and `<artifactId>`
- **THEN** an `ExternalDependency` node is created with ID `ext:<groupId>:<artifactId>` and matching properties

#### Scenario: Gradle dependency extracted
- **WHEN** `build.gradle` or `build.gradle.kts` declares a dependency
- **THEN** an equivalent `ExternalDependency` node is created

### Requirement: Maven/Gradle dependencies SHALL be connected to the graph via imports edges
Each `ExternalDependency` node produced from a Maven or Gradle dependency declaration SHALL be reachable from at least one `Package` node: when a Java source file's `import` statement resolves to an external (non-repo-internal) type, the extractor SHALL create an `imports` edge from the importing file's `Package` node to the corresponding `ExternalDependency` node, creating that node on the fly (tagged `Properties["language"] = "java"`) if no manifest-derived node already exists for it.

#### Scenario: Import of an external type creates an imports edge to its dependency node
- **WHEN** a Java file contains `import org.springframework.stereotype.Service;` and `org.springframework` is not a package produced by this repository's own source
- **THEN** an `imports` edge is created from that file's `Package` node to an `ExternalDependency` node representing `org.springframework`

#### Scenario: Internal import does not create an ExternalDependency edge
- **WHEN** a Java file imports a type from a package also produced by this repository's own source
- **THEN** the `imports` edge targets that internal `Package` node, not an `ExternalDependency` node

### Requirement: Non-fatal parse errors SHALL be collected as warnings
Extraction SHALL continue past files that cannot be fully parsed, collecting a warning per such file rather than aborting the whole extraction.

#### Scenario: Malformed file does not abort extraction
- **WHEN** one `.java` file in the repository fails to parse
- **THEN** the extractor still returns nodes/edges extracted from the remaining files, plus a warning identifying the failed file

## Edge Cases

- Anonymous classes are extracted as `Struct` nodes with a generated name (`AnonymousClass$1`).
- Inner classes are extracted with a `$` separator in their name (`OuterClass.InnerClass`).
- Generics are ignored during type extraction (type erasure).
- Custom annotations are extracted as `Decorator` nodes.
- Lombok annotations (`@Data`, `@Getter`, `@Setter`) are extracted as properties on the class and do not generate synthetic method nodes.
- Multiple annotations on the same class are all extracted.
