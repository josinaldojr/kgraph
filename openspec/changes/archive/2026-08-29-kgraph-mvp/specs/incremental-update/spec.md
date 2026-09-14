## ADDED Requirements

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
