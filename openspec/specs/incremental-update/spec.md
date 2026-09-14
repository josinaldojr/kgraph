# incremental-update

## Purpose

TBD - extracted from kgraph-mvp change. Reprocesses only the files changed since the last processed commit and invalidates the summaries of changed nodes and their direct neighbors, without a full rebuild or eager resummarization.

## Requirements

### Requirement: Diff-scoped reprocessing
`Update` SHALL identify changed files via `git diff --name-only` against the last processed commit and SHALL reprocess (re-parse and re-extract) only the nodes belonging to those files.

#### Scenario: Single-file change
- **WHEN** a commit changes exactly one source file since the last processed commit
- **THEN** `Update` re-parses only that file and leaves nodes from unchanged files untouched

### Requirement: Direct-neighbor summary invalidation
`Update` SHALL mark stale the summaries of changed nodes and their direct (one-hop) graph neighbors, and SHALL NOT eagerly regenerate those summaries as part of the update call.

#### Scenario: Changed function invalidates itself and direct callers
- **WHEN** a function's signature or body changes
- **THEN** its own summary and the summaries of its direct callers and callees are marked stale, unrelated nodes elsewhere in the graph are not, and no summarization API calls are made until those stale summaries are next needed

### Requirement: Update completes without full rebuild
`Update` SHALL NOT re-parse or re-extract any file outside the diff's changed-file set.

#### Scenario: Unrelated files skipped
- **WHEN** `Update` runs after a change touching a small subset of the repository's files
- **THEN** files outside that changed-file set are not re-parsed and their nodes' `updated_at` timestamps are unchanged

### Requirement: Changed files SHALL be routed to the correct per-language extractor by extension
`Update` SHALL route each changed file to its language's extractor based on file extension: `.go` to the Go extractor, `.java` to the Java extractor, `.ts`/`.tsx` to the TypeScript extractor, `.js`/`.jsx` to the JavaScript extractor, `.py` to the Python extractor, and `.sql` to the existing migration extraction path. A changed file with an extension matching none of these SHALL be skipped with a warning rather than aborting the update.

#### Scenario: Go file routed to Go extractor
- **WHEN** a changed file has the extension `.go`
- **THEN** it is passed to the Go extractor's `ExtractPackages`, matching existing (pre-multi-language) behavior

#### Scenario: Java file routed to Java extractor
- **WHEN** a changed file has the extension `.java`
- **THEN** it is passed to the Java extractor's `ExtractPackages`

#### Scenario: TypeScript file routed to TypeScript extractor
- **WHEN** a changed file has the extension `.ts` or `.tsx`
- **THEN** it is passed to the TypeScript extractor's `ExtractPackages`

#### Scenario: JavaScript file routed to JavaScript extractor
- **WHEN** a changed file has the extension `.js` or `.jsx`
- **THEN** it is passed to the JavaScript extractor's `ExtractPackages`

#### Scenario: Python file routed to Python extractor
- **WHEN** a changed file has the extension `.py`
- **THEN** it is passed to the Python extractor's `ExtractPackages`

#### Scenario: SQL migration routed unchanged
- **WHEN** a changed file has the extension `.sql`
- **THEN** it continues to be processed by the existing migration extraction path, with no regression from pre-multi-language behavior

#### Scenario: Unrecognized extension skipped
- **WHEN** a changed file's extension matches no known language or `.sql`
- **THEN** `Update` skips the file, emits a warning, and continues processing the remaining changed files

### Requirement: Changed files SHALL be scoped per language before extraction
For each language present among the changed files, `Update` SHALL group that language's changed files by directory/package and invoke that language's extractor's `ExtractPackages(repoPath, patterns, knownInternal)` with the resulting patterns, rather than reprocessing the whole repository.

#### Scenario: Changed files grouped by directory per language
- **WHEN** changed files include multiple Java files under the same package directory
- **THEN** `Update` derives a single scoped pattern for that directory and calls the Java extractor's `ExtractPackages` once with it, rather than once per file

#### Scenario: Multiple languages changed in one update
- **WHEN** a single update touches both Go and TypeScript files
- **THEN** `Update` computes scoped patterns per language independently and calls each language's `ExtractPackages` with only that language's patterns

### Requirement: knownInternal SHALL be filtered per language
`Update` SHALL construct the `knownInternal` map passed to each language's `ExtractPackages` from the existing graph's `Package` nodes, filtered to only those nodes whose `language` property matches the language being extracted.

#### Scenario: Package from a different language excluded
- **WHEN** the existing graph contains `Package` nodes for both Go and Java, and `Update` is extracting Java changes
- **THEN** the `knownInternal` map passed to the Java extractor contains only Java package identifiers, not Go ones

### Requirement: Per-language subgraphs SHALL be merged before saving
`Update` SHALL merge the subgraphs produced by each invoked language extractor into a single graph (per the multi-language-merge capability) before persisting the update.

#### Scenario: Two languages changed in one commit
- **WHEN** an update touches both Python and TypeScript files
- **THEN** the Python and TypeScript extractors' resulting subgraphs are merged into one graph before it is saved to the store

### Requirement: Stale invalidation SHALL apply across all languages
The existing direct-neighbor stale-invalidation logic SHALL mark stale the summaries of changed nodes and their direct neighbors regardless of which language produced those nodes.

#### Scenario: Neighbor in a different language marked stale
- **WHEN** a changed Java node has a direct graph neighbor that is a TypeScript node (e.g. via a cross-language edge)
- **THEN** the TypeScript neighbor's summary is marked stale using the same logic as same-language neighbors

## Edge Cases

- A renamed file (`git mv`) is detected as a delete plus an add and is handled correctly by the existing diff-scoped reprocessing.
- A file moved between packages results in the old node being deleted and a new node being created.
- A language newly added to the project (e.g. a new `pom.xml` appears) is not activated by `Update`; its extractor is only invoked starting with the next full build.
- A language removed from the project leaves its existing nodes in the graph untouched until the next full build.
