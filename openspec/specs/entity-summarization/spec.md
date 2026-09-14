# entity-summarization

## Purpose

TBD - extracted from kgraph-mvp change. Provides a provider-agnostic export/apply pipeline for summarizing graph nodes: listing nodes whose current content hash lacks a summary, and hash-validated ingestion of externally-produced summaries, aggregated up from node level to file and module level.

## Requirements

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

### Requirement: Hash-validated summary apply
The system SHALL accept externally-produced summaries (id, hash, summary text) and store each one only if the given hash still matches the node's current content hash, reporting (not silently dropping) any mismatch.

#### Scenario: Matching hash is applied
- **WHEN** an applied summary's hash equals the target node's current content hash
- **THEN** the summary text is stored against that node ID and hash

#### Scenario: Stale hash is rejected
- **WHEN** an applied summary's hash no longer matches the node's current content hash (the code changed since the summary was exported)
- **THEN** the summary is not stored, and the mismatch is reported to the caller rather than silently ignored

### Requirement: Hash-based summary caching
The system SHALL exclude from the pending-summary export any node whose content hash already has a stored summary, and SHALL only make a node eligible again when its content hash changes.

#### Scenario: Unchanged node on rebuild
- **WHEN** `build` or `update` runs and a node's content hash is unchanged from a prior run
- **THEN** that node does not reappear in the pending-summary export

### Requirement: Aggregated file and module summaries
The system SHALL derive a per-file summary from the summaries of the structs/interfaces/functions declared in that file, and a per-package summary from its files' summaries, without requiring raw source code as input.

#### Scenario: File summary derived from children
- **WHEN** all `Struct`/`Interface`/`Function` nodes declared in a file have summaries
- **THEN** the file-level pending-summary export's source text is built only from those child summaries, not from re-reading the file
