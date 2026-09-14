## ADDED Requirements

### Requirement: build command
The CLI SHALL provide `kgraph build <repo_path>` which runs full extraction and persistence for the given repository path and reports the count of nodes and edges created, plus how many nodes still have no summary (summarization itself is provider-driven — see `summarize pending`/`summarize apply` — not part of `build`).

#### Scenario: Successful build
- **WHEN** `kgraph build` is run against a valid Go repository path
- **THEN** the command exits with status 0 and prints the number of nodes and edges created, and the number of nodes still pending a summary

### Requirement: summarize pending command
The CLI SHALL provide `kgraph summarize pending [--level node|file|module] [--limit N]` which prints, as JSON, the nodes/files/modules lacking a current-hash summary, for an AI provider to read and summarize.

#### Scenario: Nodes without summaries are listed
- **WHEN** `kgraph summarize pending` is run against a built graph containing unsummarized nodes
- **THEN** the command prints a JSON array containing each unsummarized node's id, type, signature, file/line range, hash, and source text

### Requirement: summarize apply command
The CLI SHALL provide `kgraph summarize apply <file>` which reads a JSON array of `{id, hash, summary}` produced by a provider and stores each entry whose hash matches the node's current hash, reporting any hash mismatches rather than silently dropping them.

#### Scenario: Applying provider-written summaries
- **WHEN** `kgraph summarize apply` is run with a file containing valid `{id, hash, summary}` entries whose hashes match current node hashes
- **THEN** the command exits with status 0 and the summaries become visible in subsequent `kgraph context`/`kgraph search` output

#### Scenario: Stale entry is reported, not applied
- **WHEN** an entry's hash no longer matches its node's current hash
- **THEN** the command does not store that entry and reports it as skipped, without failing the entries that did apply cleanly

### Requirement: context command with defaults
The CLI SHALL provide `kgraph context <target> [--hops N] [--max-tokens N]`, defaulting `--hops` to 2 and `--max-tokens` to 3000 when not specified.

#### Scenario: Defaults applied
- **WHEN** `kgraph context <target>` is run without `--hops` or `--max-tokens`
- **THEN** the underlying `GetContext` call uses `hops=2` and `maxTokens=3000`

### Requirement: search command
The CLI SHALL provide `kgraph search <query>` which prints ranked matching nodes (name, type, file location, match score) to stdout, using lexical `SearchNodes`.

#### Scenario: Query returns ranked matches
- **WHEN** `kgraph search` is run with a non-empty query against a built graph
- **THEN** the command prints matching nodes ordered by descending match score

### Requirement: update command
The CLI SHALL provide `kgraph update` which runs `Update` against the current repository's git history, and SHALL fail with a clear error (not a panic or stack trace) when run outside a git repository or with no prior `build` having been run.

#### Scenario: Update outside a git repository
- **WHEN** `kgraph update` is run in a directory that is not a git repository
- **THEN** the command exits non-zero with a clear error message and does not panic
