## 1. Project setup

- [x] 1.1 Create new standalone Go module `kgraph` (`go.mod`, Go 1.22+), separate from `knowledge-cli`
- [x] 1.2 Scaffold `cmd/kgraph/main.go` and empty `internal/{parser,graph,store,summarizer,context}` packages
- [x] 1.3 Add core dependencies: `golang.org/x/tools/go/packages`, `modernc.org/sqlite`, `github.com/spf13/cobra`

## 2. Graph model (`internal/graph`)

- [x] 2.1 Define `Node{ID, Type, File, LineStart, LineEnd, Signature, Hash, Properties map[string]any}` and the node-type enum/consts (`Package`, `Struct`, `Interface`, `Function`, `Field`, `Table`, `Column`, `Endpoint`, `ExternalDependency`)
- [x] 2.2 Define `Edge{ID, Type, SrcID, DstID, Properties}` and the edge-type consts (`imports`, `calls`, `embeds`, `implements`, `has_method`, `has_field`, `maps_to_table`, `references_fk`, `reads_table`, `writes_table`, `exposes_endpoint`)
- [x] 2.3 Implement the in-memory `Graph` type (`map[string]*Node`, adjacency maps for outgoing/incoming edges) with `AddNode`, `AddEdge`, `Node(id)`, `Edges(id)` lookups
- [x] 2.4 Implement deterministic node ID generation per node type (e.g. `pkg/path.Struct`, `pkg/path.Func`, `table:name`)
- [x] 2.5 Implement content-hash computation (SHA-256 over normalized signature/body per node type)

## 3. Go source extraction (`internal/parser`)

- [x] 3.1 Walk the target repo's Go files with `go/parser`, building one `Package` node per package and `imports` edges
- [x] 3.2 Extract `Struct`/`Interface`/`Field` nodes and `has_field` edges from type declarations
- [x] 3.3 Extract `Function` nodes (including methods) with params, return types, and receiver linkage (`has_method` edge to the receiver's `Struct`)
- [x] 3.4 Load the repo with `golang.org/x/tools/go/packages` and resolve call expressions within each function body into `calls` edges
- [x] 3.5 Detect `gorm`/`db` struct tags and derive `Table`/`Column` nodes plus `maps_to_table`/`references_fk` edges
- [x] 3.6 Parse `.sql` migration files and reconcile their schema with struct-tag-derived `Table`/`Column` nodes (migration wins on conflict; conflicting struct-tag fact retained in `properties`)
- [x] 3.7 Write extraction tests against fixture Go source covering structs, methods, calls, ORM tags, and a sample migration

## 4. Storage (`internal/store`)

- [x] 4.1 Define SQLite schema (`nodes`, `edges`, `summaries`, `build_meta` tables) and migration bootstrap using `modernc.org/sqlite`
- [x] 4.2 Implement graph persistence: upsert nodes/edges keyed by content hash (no-op write when hash unchanged)
- [x] 4.3 Implement graph load: reconstruct the in-memory `Graph` from a SQLite database
- [x] 4.4 Resolve default database path (per design's open question — pin to an external per-user cache dir keyed by absolute repo path, override via `--db` flag)

## 5. Build pipeline checkpoint

- [x] 5.1 Wire `internal/parser` → `internal/graph` → `internal/store` into a `Build(repoPath)` pipeline (extraction + persistence only, no summarization yet)
- [x] 5.2 Run the pipeline end-to-end against a real Go repository and inspect the resulting `nodes`/`edges` tables for correctness
- [x] 5.3 Verify idempotency: re-running `Build` on an unchanged repo produces zero row updates

## 6. Entity summarization (`internal/summarizer`) — provider-driven, revised mid-implementation

- [x] 6.1 Implement `PendingSummaries(level string, limit int) ([]PendingSummary, error)`: query nodes/files/modules lacking a current-hash summary, with `source` = own text (node level) or concatenated child summaries (file/module level)
- [x] 6.2 Implement `ApplySummaries(entries []AppliedSummary) (ApplyResult, error)`: persist each `{id, hash, summary}` to `summaries` only if `hash` matches the node's current hash; collect mismatches instead of failing the whole batch
- [x] 6.3 Derive per-file summaries from child node summaries, and per-package summaries from file summaries (both surfaced via `PendingSummaries` once their children are summarized, per 6.1)
- [x] 6.4 Write tests verifying unchanged-hash nodes are excluded from `PendingSummaries`, and that `ApplySummaries` rejects stale-hash entries without discarding the valid ones in the same batch

## 7. Node search (`internal/summarizer` or `internal/store`) — lexical, revised mid-implementation

- [x] 7.1 Implement a lexical scorer: tokenize query + node summary (falling back to signature/name if unsummarized), score by term overlap
- [x] 7.2 Implement `SearchNodes(query string, topK int) []Node` ranking by descending lexical score, no embeddings/vector DB/external API
- [x] 7.3 Write tests covering ranked ordering and the unsummarized-node fallback to signature/name matching

## 8. Context assembly (`internal/context`)

- [x] 8.1 Implement target resolution (exact file/struct/function name match, falling back to `SearchNodes`)
- [x] 8.2 Implement BFS subgraph expansion up to `hops` edges, ranked by hop-distance then edge-type weight
- [x] 8.3 Implement summary-based rendering (signature, summary, direct relations: callers, callees, tables read/written) with no raw source
- [x] 8.4 Implement token estimation and budget-aware truncation (drop farthest-hop nodes first; keep target's own summary and direct relations)
- [x] 8.5 Implement `GetContext(target string, hops int, maxTokens int) (string, error)` with defaults `hops=2`, `maxTokens=3000`
- [x] 8.6 Write tests covering exact-match resolution, fallback-to-search resolution, and truncation under a tight token budget

## 9. Incremental update

- [x] 9.1 Implement changed-file detection via `os/exec` `git diff --name-only <last_commit>..HEAD`, reading/writing `build_meta.last_commit`
- [x] 9.2 Implement `Update(repoPath, dbPath) (UpdateResult, error)`: re-parse only the Go packages containing changed files (plus migrations if a `.sql` changed) and persist updates — named `Update`, not the original prompt's literal `UpdateFromDiff(gitDiff string)`, since the diff is fetched internally via `gitutil`, not passed in by the caller
- [x] 9.3 Implement direct-neighbor summary invalidation (mark stale in `summaries`, no eager regeneration)
- [x] 9.4 Write a test/benchmark comparing full `Build` time vs. `Update` time after a single-file change, confirming update only touches diff-scoped files

## 10. CLI (`cmd/kgraph`, cobra)

- [x] 10.1 Implement `kgraph build <repo_path>` (runs extraction + persistence only; reports node/edge counts and how many nodes still need summaries)
- [x] 10.2 Implement `kgraph update` (runs `Update` against the current repo, clear error when not in a git repo or with no prior build)
- [x] 10.3 Implement `kgraph context <target> [--hops N] [--max-tokens N]` (defaults 2 / 3000)
- [x] 10.4 Implement `kgraph search <query>` (prints ranked node name/type/file/score)
- [x] 10.5 Implement `kgraph summarize pending` and `kgraph summarize apply <file>` (JSON export/import round trip for provider-driven summarization, per design.md Decision 6)

## 11. End-to-end validation

- [x] 11.1 Run `kgraph build` against a real Go repository from the workspace portfolio and confirm zero errors — used `anti-fraudeiro` (277 nodes, 317 edges, zero errors)
- [x] 11.2 Acting as the provider: run `kgraph summarize pending`, write summaries for the returned nodes, run `kgraph summarize apply` to store them — 12 real nodes summarized from actual source, 12/12 applied
- [x] 11.3 Run `kgraph context <some-struct-or-function>` and confirm output is readable, a few hundred to a few thousand tokens, and includes purpose + direct relations — `kgraph context "Handler.FraudScore"` produced a ~600-token, readable 2-hop view (struct purpose, its methods/fields, FraudScore's callees)
- [x] 11.4 Make a small code change, run `kgraph update`, and measure/compare elapsed time against a full `kgraph build` to confirm incremental reprocessing is materially faster — covered by `TestUpdateFasterThanFullRebuild` (259ms update vs 689ms full rebuild across 40 packages) using a throwaway git fixture rather than the user's real repos, to avoid committing test changes into them
