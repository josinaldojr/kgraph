## ADDED Requirements

### Requirement: package.json dependencies SHALL be connected to the graph via imports edges
Each `ExternalDependency` node produced from a `package.json` dependency entry SHALL be reachable from at least one `Package` node: when a TypeScript/JavaScript `import`/`require` statement resolves to an external (non-repo-internal) module specifier, the extractor SHALL create an `imports` edge from the importing file's `Package` node to the corresponding `ExternalDependency` node, creating that node on the fly (tagged with the extractor's actual running language, per the existing `multi-language-merge` requirement) if no manifest-derived node already exists for it.

#### Scenario: Import of an external module creates an imports edge to its dependency node
- **WHEN** a TypeScript file contains `import express from 'express';` and `express` is not a module produced by this repository's own source
- **THEN** an `imports` edge is created from that file's `Package` node to an `ExternalDependency` node representing `express`

#### Scenario: Relative import does not create an ExternalDependency edge
- **WHEN** a file imports via a relative specifier (e.g. `import { foo } from './bar'`)
- **THEN** the `imports` edge targets the internal `Package` node for `./bar`, not an `ExternalDependency` node
