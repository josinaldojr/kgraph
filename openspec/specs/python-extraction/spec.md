# python-extraction

## Purpose

Extracts a code knowledge graph from Python repositories: modules, packages, classes, functions, imports, inheritance, calls, decorators (Flask, FastAPI, SQLAlchemy), and dependencies. This extractor is regex-based and best-effort (not AST-based), so accuracy and completeness are lower-fidelity than the Go extractor.

## Requirements

### Requirement: Declaration extraction SHALL cover packages, modules, classes, and functions
The Python extractor SHALL treat a directory containing `__init__.py` as a `Package` node (ID `pkg:<package>`), a `.py` file as a `Package` node representing a module (ID `pkg:<package>.<module>`), and SHALL produce `Struct` nodes for classes (with property `dataclass=true` for `@dataclass`-decorated classes), `Interface` nodes for ABC/Protocol-based classes, `Enum` nodes for enum classes, `Function` nodes for module-level functions and methods (with property `static=true` for `@staticmethod` and `classmethod=true` for `@classmethod`), and `Field` nodes for properties and class variables (with property `classvar=true` for class-level variables).

#### Scenario: Package directory produces a Package node
- **WHEN** the extractor encounters a directory containing `__init__.py`
- **THEN** it creates a `Package` node with ID `pkg:<package path>`

#### Scenario: Module file produces a Package node
- **WHEN** the extractor encounters a `.py` file
- **THEN** it creates a `Package` node representing that module

#### Scenario: Dataclass flagged
- **WHEN** a class is decorated `@dataclass`
- **THEN** the resulting `Struct` node has `Properties["dataclass"] = true`

#### Scenario: ABC or Protocol produces an Interface node
- **WHEN** a class inherits from `ABC` or `Protocol`
- **THEN** an `Interface` node is created instead of a `Struct` node

#### Scenario: Static method flagged
- **WHEN** a method is decorated `@staticmethod`
- **THEN** the resulting `Function` node has `Properties["static"] = true`

#### Scenario: Class method flagged
- **WHEN** a method is decorated `@classmethod`
- **THEN** the resulting `Function` node has `Properties["classmethod"] = true`

#### Scenario: Class variable flagged
- **WHEN** a class declares a variable at class level (not inside `__init__`)
- **THEN** the resulting `Field` node has `Properties["classvar"] = true`

### Requirement: Relationship extraction SHALL cover imports, inheritance, calls, and membership
The Python extractor SHALL create `imports` edges from `import` and `from ... import ...` statements, `extends` edges from `class Foo(Bar):` base-class declarations, `implements` edges when the base is `ABC` or `Protocol`, best-effort `calls` edges from resolvable call expressions, and `has_method` edges linking methods to their declaring class.

#### Scenario: Import statement creates imports edge
- **WHEN** a module contains `import mypackage.foo`
- **THEN** an `imports` edge is created to `mypackage.foo`

#### Scenario: From-import creates imports edge
- **WHEN** a module contains `from mypackage.foo import Bar`
- **THEN** an `imports` edge is created to `mypackage.foo.Bar`

#### Scenario: Class inheritance creates extends edge
- **WHEN** a class declares `class Foo(Bar):` where `Bar` is not `ABC` or `Protocol`
- **THEN** an `extends` edge is created from `Foo` to `Bar`

#### Scenario: ABC/Protocol base creates implements edge
- **WHEN** a class declares `class Foo(ABC):` or `class Foo(Protocol):`
- **THEN** an `implements` edge is created from `Foo` to the ABC/Protocol interface node

#### Scenario: Relative imports resolved relative to package
- **WHEN** a module uses a relative import (e.g. `from . import foo` or `from ..pkg import bar`)
- **THEN** the extractor resolves the target relative to the importing module's package location

### Requirement: Best-effort call resolution
The Python extractor SHALL resolve `self.method()` to a method on the same class, `cls.method()` to a classmethod on the same class, `module.function()` to a function in an imported module, and unqualified `function()` to an imported or locally defined function, on a best-effort basis without full type inference.

#### Scenario: Self-call resolves within same class
- **WHEN** a method body contains `self.save()`
- **THEN** a `calls` edge is created to the `save` method on the same class, if one exists

#### Scenario: Module-qualified call resolves to imported module
- **WHEN** a function body contains `module.function()` where `module` was imported
- **THEN** a `calls` edge is created to `function` in that imported module, if resolvable

### Requirement: Flask and FastAPI routes SHALL be extracted as routed edges
The Python extractor SHALL recognize `@app.route(...)` (Flask) and `@app.get(...)`/`@app.post(...)`/etc. (FastAPI) decorators on functions, creating an `Endpoint` node for the declared path and HTTP method(s), with a `routed` edge from the decorated function to that `Endpoint` node.

#### Scenario: Flask route creates routed edge
- **WHEN** a function is decorated `@app.route('/users', methods=['GET'])`
- **THEN** an `Endpoint` node `endpoint:GET /users` is created and a `routed` edge is created from the function to it

#### Scenario: FastAPI route creates routed edge
- **WHEN** a function is decorated `@app.get("/users/{user_id}")`
- **THEN** an `Endpoint` node `endpoint:GET /users/{user_id}` is created and a `routed` edge is created from the function to it

### Requirement: SQLAlchemy models SHALL be extracted as maps_to_table relationships
The Python extractor SHALL recognize SQLAlchemy-style model classes (declaring `__tablename__` and `Column(...)` attributes), creating a `Table` node named per `__tablename__`, a `maps_to_table` edge from the class to that `Table` node, and a `Field` node per `Column(...)` attribute carrying `column` and `type` properties.

#### Scenario: Tablename creates maps_to_table edge
- **WHEN** a class declares `__tablename__ = 'users'`
- **THEN** a `Table` node `table:users` is created and a `maps_to_table` edge is created from the class to it

#### Scenario: Column attribute creates Field with column property
- **WHEN** a class attribute is assigned `Column(Integer, primary_key=True)`
- **THEN** a `Field` node is created with `Properties["column"]` set to the attribute name and `Properties["type"]` set to the column type

### Requirement: Dependencies SHALL be extracted from requirements.txt, pyproject.toml, or setup.py
The Python extractor SHALL parse `requirements.txt`, `pyproject.toml`, or `setup.py` to produce `ExternalDependency` nodes carrying a `version` property when a version constraint is present.

#### Scenario: Requirements.txt entry extracted
- **WHEN** `requirements.txt` contains `flask>=2.0.0`
- **THEN** an `ExternalDependency` node `ext:flask` is created with `Properties["version"] = ">=2.0.0"`

### Requirement: requirements.txt/pyproject.toml dependencies SHALL be connected to the graph via imports edges
Each `ExternalDependency` node produced from a `requirements.txt` or `pyproject.toml` dependency entry SHALL be reachable from at least one `Package` node: when a Python `import`/`from ... import` statement resolves to an external (non-repo-internal) module, the extractor SHALL create an `imports` edge from the importing module's `Package` node to the corresponding `ExternalDependency` node, creating that node on the fly (tagged `Properties["language"] = "python"`) if no manifest-derived node already exists for it, instead of creating a bare `Package` node for the external target.

#### Scenario: Import of an external module creates an imports edge to its dependency node
- **WHEN** a Python file contains `import flask` and `flask` is not a module produced by this repository's own source
- **THEN** an `imports` edge is created from that file's `Package` node to an `ExternalDependency` node representing `flask`, and no bare, property-less `Package` node is created for `flask`

#### Scenario: Internal import does not create an ExternalDependency edge
- **WHEN** a Python file imports a module also produced by this repository's own source
- **THEN** the `imports` edge targets that internal `Package` node, not an `ExternalDependency` node

### Requirement: Non-fatal parse errors SHALL be collected as warnings
Extraction SHALL continue past files that cannot be fully parsed, collecting a warning per such file rather than aborting the whole extraction.

#### Scenario: Malformed file does not abort extraction
- **WHEN** one `.py` file in the repository fails to parse
- **THEN** the extractor still returns nodes/edges extracted from the remaining files, plus a warning identifying the failed file

## Edge Cases

- Stacked decorators are all extracted.
- Anonymous functions (lambdas) are ignored, since they lack a static name.
- List comprehensions are ignored, since they are not declarations.
- Type hints are extracted as properties but not resolved against the type system.
- `__init__.py` is treated as a module node like any other `.py` file.
