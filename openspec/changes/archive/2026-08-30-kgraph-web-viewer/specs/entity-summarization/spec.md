## MODIFIED Requirements

### Requirement: Pending-summary export
The system SHALL provide a way to list `Struct`/`Interface`/`Function`/`Table`/`Endpoint`/`ExternalDependency` nodes (and, once their children are summarized, files and modules) that lack a summary for their current content hash, exporting for each one its ID, type, signature (where applicable), file/line range (where applicable), hash, and source text. Source text is the node's own text at `node` level — for `Struct`/`Interface`/`Function` this is their source span; for `Table` it is its column list plus incoming foreign-key references; for `Endpoint` it is its HTTP method, path, and handler signature; for `ExternalDependency` it is its module path and declared version — or, at `file`/`module` level, its children's concatenated summaries. `Field` and `Column` nodes SHALL NOT be included in the pending-summary export; they are described only as part of their owning `Struct`/`Table`'s summary.

#### Scenario: Node without a cached summary is listed
- **WHEN** a `Struct`, `Interface`, or `Function` node has no stored summary matching its current content hash
- **THEN** that node appears in the pending-summary export with its ID, signature, file/line range, hash, and source text

#### Scenario: File pending only once its children are summarized
- **WHEN** a file's `Struct`/`Interface`/`Function` nodes all have current summaries
- **THEN** that file becomes eligible for a file-level pending summary export, built from its children's summaries rather than raw source

#### Scenario: Table without a cached summary is listed
- **WHEN** a `Table` node has no stored summary matching its current content hash
- **THEN** that node appears in the pending-summary export with its ID, hash, and source text built from its column list and incoming foreign-key edges, with no file/line range required

#### Scenario: Endpoint without a cached summary is listed
- **WHEN** an `Endpoint` node has no stored summary matching its current content hash
- **THEN** that node appears in the pending-summary export with its ID, hash, and source text built from its HTTP method, path, and handler function's signature

#### Scenario: External dependency without a cached summary is listed
- **WHEN** an `ExternalDependency` node has no stored summary matching its current content hash
- **THEN** that node appears in the pending-summary export with its ID, hash, and source text built from its module path and declared version

#### Scenario: Field and Column nodes never appear on their own
- **WHEN** the pending-summary export is generated at `node` level
- **THEN** no `Field` or `Column` node appears in it, regardless of whether its owning `Struct`/`Table` has a current summary
