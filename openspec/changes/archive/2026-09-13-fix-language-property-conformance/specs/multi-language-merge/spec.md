## MODIFIED Requirements

### Requirement: Every extracted node SHALL carry a language property
Each node produced by a language extractor SHALL have its `Properties["language"]` set to that extractor's language, so nodes can be filtered by language in the frontend and in context generation after merging. This applies to every node type an extractor produces, including `ExternalDependency`, `Table`, and `Column` nodes, not only the primary language-native construct types (e.g. `Struct`, `Function`).

#### Scenario: Node tagged with source language
- **WHEN** the Java extractor produces a `Struct` node
- **THEN** that node's `Properties["language"]` SHALL equal `"java"`

#### Scenario: External dependency tagged with the extractor's actual language
- **WHEN** an extractor running in JavaScript mode (not TypeScript mode) produces an `ExternalDependency` node from a `package.json` dependency
- **THEN** that node's `Properties["language"]` SHALL equal `"javascript"`, not `"typescript"`

#### Scenario: Table and Column nodes tagged regardless of origin
- **WHEN** the Go extractor produces a `Table` or `Column` node, whether inferred from a struct tag or from a SQL migration
- **THEN** that node's `Properties["language"]` SHALL equal `"go"`
