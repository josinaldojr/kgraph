# typescript-extraction

## Purpose

Extracts a code knowledge graph from TypeScript and JavaScript repositories: modules, classes, interfaces, types, functions, imports, inheritance, calls, decorators (NestJS, TypeORM), and package.json dependencies. This extractor is regex-based and best-effort (not AST-based), so accuracy and completeness are lower-fidelity than the Go extractor.

## Requirements

### Requirement: Declaration extraction SHALL cover modules, types, and functions
The TypeScript/JavaScript extractor SHALL produce a `Package` node per module file, `Struct` nodes for classes (with property `abstract=true` for abstract classes), `Interface` nodes for interfaces, `TypeAlias` nodes for `type X = ...` declarations, `Enum` nodes for enum declarations, and `Function` nodes for functions, methods, and arrow functions assigned to a `const`.

#### Scenario: Class produces a Struct node
- **WHEN** the extractor encounters a `class Foo { ... }` declaration
- **THEN** it creates a `Struct` node with an ID scoped to the module (e.g. `<module>.Foo`)

#### Scenario: Abstract class flagged
- **WHEN** a class is declared `abstract class Foo`
- **THEN** the resulting `Struct` node has `Properties["abstract"] = true`

#### Scenario: Interface produces an Interface node
- **WHEN** the extractor encounters an `interface Foo { ... }` declaration
- **THEN** it creates an `Interface` node

#### Scenario: Type alias produces a TypeAlias node
- **WHEN** the extractor encounters a `type Status = ...` declaration
- **THEN** it creates a `TypeAlias` node

#### Scenario: Enum produces an Enum node
- **WHEN** the extractor encounters an `enum Role { ... }` declaration
- **THEN** it creates an `Enum` node

#### Scenario: Arrow function assigned to const produces a Function node
- **WHEN** the extractor encounters `const formatUser = (...) => { ... }`
- **THEN** it creates a `Function` node for `formatUser`

### Requirement: Relationship extraction SHALL cover imports, inheritance, calls, and membership
The extractor SHALL create `imports` edges from ES `import` statements and CommonJS `require(...)` calls, `extends` edges from class/interface inheritance, `implements` edges from `class Foo implements Bar`, best-effort `calls` edges, and `has_method` edges linking methods to their declaring class.

#### Scenario: ES import creates imports edge
- **WHEN** a module contains `import { Foo } from './foo'`
- **THEN** an `imports` edge is created to the `./foo` module

#### Scenario: CommonJS require creates imports edge
- **WHEN** a module contains `const foo = require('./foo')`
- **THEN** an `imports` edge is created to the `./foo` module

#### Scenario: Class extends creates extends edge
- **WHEN** a class declares `class Foo extends Bar`
- **THEN** an `extends` edge is created from `Foo` to `Bar`

#### Scenario: Interface extends creates extends edge
- **WHEN** an interface declares `interface Foo extends Bar`
- **THEN** an `extends` edge is created from `Foo` to `Bar`

#### Scenario: Class implements creates implements edge
- **WHEN** a class declares `class Foo implements Bar`
- **THEN** an `implements` edge is created from `Foo` to `Bar`

### Requirement: Best-effort call resolution
The extractor SHALL resolve `this.method()` to a method on the same class and unqualified `foo()` to a locally defined or imported function, on a best-effort basis without full type inference.

#### Scenario: This-call resolves within same class
- **WHEN** a method body contains `this.save()`
- **THEN** a `calls` edge is created to the `save` method on the same class, if one exists

### Requirement: NestJS decorators SHALL be extracted as decorated/routed/injected relationships
The extractor SHALL recognize `@Injectable()` and `@Controller(...)` as `decorated` edges, `@Get(...)`/`@Post(...)` and similar HTTP-method decorators as `routed` edges to `Endpoint` nodes (combining the controller's base path with the method's path), and `@Inject()` as an `injected` edge.

#### Scenario: Controller decorator creates decorated edge
- **WHEN** a class is decorated `@Controller('users')`
- **THEN** a `decorated` edge is created from the class to a `Decorator` node, and the class's base path property is set to `users`

#### Scenario: HTTP method decorator creates routed edge
- **WHEN** a method within a `@Controller('users')` class is decorated `@Get(':id')`
- **THEN** an `Endpoint` node `endpoint:GET users/:id` is created and a `routed` edge is created from the method to it

#### Scenario: Inject decorator creates injected edge
- **WHEN** a constructor parameter or property is decorated `@Inject()`
- **THEN** an `injected` edge is created from the declaring class to the injected type

### Requirement: TypeORM decorators SHALL be extracted as maps_to_table relationships
The extractor SHALL recognize `@Entity()` as a `maps_to_table` edge to a `Table` node, and `@Column()` on a property as a `has_field` edge to a `Column` node under that table.

#### Scenario: Entity decorator creates maps_to_table edge
- **WHEN** a class is decorated `@Entity()`
- **THEN** a `Table` node is created and a `maps_to_table` edge is created from the class to it

#### Scenario: Column decorator creates table column
- **WHEN** a property within an `@Entity()` class is decorated `@Column()`
- **THEN** a `Column` node is created under that entity's `Table` node, linked via `has_field`

### Requirement: Package.json dependencies SHALL be extracted as ExternalDependency nodes
The extractor SHALL parse `package.json` dependency entries (`dependencies` and `devDependencies`) into `ExternalDependency` nodes.

#### Scenario: Dependency entry extracted
- **WHEN** `package.json` lists a package under `dependencies`
- **THEN** an `ExternalDependency` node is created for it

### Requirement: package.json dependencies SHALL be connected to the graph via imports edges
Each `ExternalDependency` node produced from a `package.json` dependency entry SHALL be reachable from at least one `Package` node: when a TypeScript/JavaScript `import`/`require` statement resolves to an external (non-repo-internal) module specifier, the extractor SHALL create an `imports` edge from the importing file's `Package` node to the corresponding `ExternalDependency` node, creating that node on the fly (tagged with the extractor's actual running language, per the existing `multi-language-merge` requirement) if no manifest-derived node already exists for it.

#### Scenario: Import of an external module creates an imports edge to its dependency node
- **WHEN** a TypeScript file contains `import express from 'express';` and `express` is not a module produced by this repository's own source
- **THEN** an `imports` edge is created from that file's `Package` node to an `ExternalDependency` node representing `express`

#### Scenario: Relative import does not create an ExternalDependency edge
- **WHEN** a file imports via a relative specifier (e.g. `import { foo } from './bar'`)
- **THEN** the `imports` edge targets the internal `Package` node for `./bar`, not an `ExternalDependency` node

### Requirement: Non-fatal parse errors SHALL be collected as warnings
Extraction SHALL continue past files that cannot be fully parsed, collecting a warning per such file rather than aborting the whole extraction.

#### Scenario: Malformed file does not abort extraction
- **WHEN** one `.ts` or `.js` file in the repository fails to parse
- **THEN** the extractor still returns nodes/edges extracted from the remaining files, plus a warning identifying the failed file

## Edge Cases

- Arrow functions assigned to a `const` are extracted as `Function` nodes.
- Destructuring imports (`import { A, B } from './x'`) produce multiple import edges.
- Dynamic imports (`import(...)`) are ignored, since they are not statically resolvable.
- Re-exports are extracted as both an import and a re-export relationship.
- Generics are ignored during type extraction.
- Custom decorators are extracted as `Decorator` nodes.
