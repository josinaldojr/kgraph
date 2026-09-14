## Context

The workspace already contains `knowledge-cli` (binary `kv`), a Go CLI/MCP server that persists *decision* memory (OpenSpec before/after snapshots) in SQLite (`modernc.org/sqlite`, cgo-free) and already calls an LLM in batch for summarization (`internal/wiki` uses Gemini for `kv wiki compile`). `kgraph` targets a different kind of memory — *code-structure* memory (entities, relationships, what a function touches) — and was deliberately scoped as a **standalone module**, not a new feature set inside `kv`, so the two memory domains stay independent (mirroring `kv`'s own documented rule against silently bridging its MCP-memory and legacy-harness feature sets). `kgraph` may later become a context source `kv` calls into, but that integration is out of scope for this change.

The rest of the workspace is a mixed-language portfolio (4 Go projects, 2 Rust, 1 Node, 1 mixed), but Go is the majority and is also the language `kgraph` itself is written in — which opens a specific implementation shortcut discussed below.

## Goals / Non-Goals

**Goals:**
- `kgraph build <repo_path>` extracts a typed graph (packages, structs, interfaces, functions, fields, ORM-mapped tables/columns, endpoints) plus call/embed/implements edges from a real Go repository, with no errors, and persists it to SQLite.
- `kgraph context <target>` produces a readable, summary-based context string (hundreds to a few thousand tokens) — purpose + direct relations — sufficient for an AI reviewer to understand a component without reading its source.
- `kgraph update` after a small change reprocesses measurably less than `kgraph build` from scratch (only touched files + directly dependent summaries).
- `kgraph search <query>` returns relevant nodes ranked by lexical match against their summaries/names, not just raw substring match.
- Summaries are produced by whatever AI provider is already driving the session (Claude Code or similar) — `kgraph` itself never calls an LLM API and needs no API key to build/update/context/search.

**Non-Goals (this change):**
- Full type inference / whole-program pointer analysis (best-effort resolution via `go/packages` only).
- Multi-language support in the same graph (Go only; tree-sitter path deferred — see Decisions).
- Web UI, Neo4j/Kùzu, or any dedicated vector database.
- Embedding-based semantic search (dropped in favor of lexical search — see Decision 7; revisit only if lexical search proves insufficient).
- Integration with `kv`/`knowledge-cli`'s MCP memory (`MemoryRetriever`, etc.) — noted as a natural follow-up, not built here.
- A background daemon or watch mode — `build`/`update`/`context`/`search` are one-shot CLI invocations. `summarize pending`/`summarize apply` are also one-shot: no persistent MCP server in this MVP (see Decision 6).

## Decisions

**1. `go/ast` + `go/parser` + `golang.org/x/tools/go/packages` instead of `tree-sitter`.**
The upstream prompt's own "Observação sobre linguagem" flags this as the simplifying path when the target is Go specifically. Two concrete reasons beyond "simpler": (a) `github.com/smacker/go-tree-sitter` grammars are C, so adopting it typically reintroduces a cgo build requirement — directly undermining the reason `modernc.org/sqlite` was chosen over `mattn/go-sqlite3` (cgo-free builds/distribution); (b) `go/packages` gives real, compiler-grade call-edge resolution instead of the heuristic AST-walk the original prompt accepted as an MVP compromise. Trade-off: no path to Rust/TS repos in the portfolio (`frac`, `zamp`, `open-memory`) without a second parsing strategy later — accepted, since multi-language is explicitly out of scope.

**2. Standalone Go module, own SQLite database, own cobra CLI.**
Confirmed with the user: `kgraph` is not a `kv` subcommand. This avoids two frictions: `kv`'s `CLAUDE.md` explicitly directs new subcommands toward its flat `switch`-over-`os.Args` pattern instead of a CLI framework (cobra would be an inconsistent addition there), and mixing code-structure memory into `kv`'s store schema would blur a boundary `kv` is deliberately strict about elsewhere.

**3. Graph representation: plain maps in `internal/graph`, no graph library.**
`map[string]*Node` and `map[string][]*Edge` (adjacency by source node ID), matching the MVP-appropriate scope in the prompt. `Node.ID` is a stable, deterministic string (e.g. `pkg/path.Struct` or `pkg/path.Func` or `table:orders`) so rebuilds are idempotent and diff-friendly.

**4. SQLite schema: two tables, typed + JSON properties.**
```
nodes(id TEXT PRIMARY KEY, type TEXT, file TEXT, line_start INT, line_end INT,
      signature TEXT, hash TEXT, properties TEXT /* JSON */, updated_at INTEGER)
edges(id TEXT PRIMARY KEY, type TEXT, src_id TEXT, dst_id TEXT,
      properties TEXT /* JSON */)
summaries(node_id TEXT, level TEXT /* node|file|module */, hash TEXT,
          summary TEXT, model TEXT, stale INTEGER, PRIMARY KEY(node_id, level))
build_meta(repo_path TEXT PRIMARY KEY, last_commit TEXT, last_build_at INTEGER)
```
`summaries` is keyed by content `hash`, not just `node_id`, so a changed node's stale summary stays queryable (for diffing) until overwritten, and unchanged nodes never show up in `summarize pending` again. `model` records what produced the summary (e.g. a provider/session identifier) — repurposed from the original embeddings-era design, not tied to any specific LLM vendor.

**5. Change detection: per-node content hash, not whole-file hash.**
Each node's hash is SHA-256 over its normalized AST source span (signature + body for functions, field list for structs). This is what lets `Update` invalidate only the struct/function whose text actually changed, not everything in a touched file.

**6. Summarization: provider-driven export/apply, not a direct LLM API client.**
Revised mid-implementation: the user does not want `kgraph` to hold its own paid LLM API key. Instead, `kgraph` exposes the work as two CLI commands that let whatever AI provider is already driving the session (Claude Code, Codex, OpenCode, ...) do the actual summarizing — the same "provider" concept `kv` already uses, just without a live MCP protocol server in this MVP:
- `kgraph summarize pending [--level node|file|module] [--limit N]` prints a JSON array of `{id, type, signature, file, line_start, line_end, hash, source}` for nodes/files/modules lacking a current-hash summary. `source` is the node's own text for `node`-level entries, or the concatenated child summaries for `file`/`module`-level entries (so aggregation never needs raw source, per the original spec's "zoom out" design).
- `kgraph summarize apply <file.json>` reads back a JSON array of `{id, hash, summary}` and upserts each into `summaries` **only if `hash` still matches the node's current hash** — protecting against writing a stale summary if the code changed between `pending` and `apply`. Mismatches are reported, not silently dropped.
This keeps the cache-by-hash behavior from the original design intact (task 6.2/6.4's requirements are unchanged); only who calls the LLM changes. A full `kgraph mcp` server mirroring `kv mcp` (so a provider can pull/push summaries live instead of via a file round-trip) is a natural v2, not built here.

**7. Search: lexical scoring over summaries/names, no embeddings.**
Also revised: with no LLM API key, there's no embedding API either (Voyage AI required one). `SearchNodes(query string, topK int) []Node` instead tokenizes the query and scores each node by term overlap against its summary (falling back to its signature/name if unsummarized yet), ranking by descending score. No vector storage, no cosine similarity, no external dependency. This is a real capability reduction versus the original "semantic search" framing — acceptable for v1 because `GetContext`'s primary resolution path is exact name/file match (Decision 8); lexical `SearchNodes` is the fallback for fuzzy/topic queries, not the primary interface.

**8. `GetContext`: BFS with hop budget, then greedy token-budgeted truncation.**
Resolve `target` (exact file/struct/function name match first, falling back to `SearchNodes` for fuzzy/topic queries) → BFS outward up to `hops` edges, ranking discovered nodes by hop-distance then edge-type weight (e.g. `calls`/`has_method` weighted above `imports`) → render nearest-first using summaries + direct relation lists (callers, callees, tables read/written) → stop rendering once the running token estimate would exceed `maxTokens`. Token estimate is an approximation (no Go-native Anthropic tokenizer); implemented as `len(s)/3 + 1` — chars/3 rather than the more typical chars/4, deliberately biased up since code text runs denser than prose, and documented as approximate in `internal/context.EstimateTokens`.

**9. `Update`: `git diff --name-only <last_commit>..HEAD` via `os/exec`, not `go-git`.**
Simpler dependency footprint; `go-git` isn't needed for a name-only diff. `build_meta.last_commit` tracks the last processed commit so `update` with no args diffs against it. Invalidation is direct-neighbor-only (one hop) and lazy: neighbors are flagged stale in `summaries` but only regenerated the next time `build`/`update`/`context` actually needs them — not eagerly, to avoid cascades from widely-called functions triggering large re-summarization runs.

## Risks / Trade-offs

- **[Risk]** `go/packages` call resolution still misses interface/dynamic-dispatch call edges → **Mitigation**: documented as best-effort (matches the original MVP's own "no precisa de resolução perfeita" acceptance), not a correctness bug; revisit only if review quality data shows it matters.
- **[Risk]** Struct-tag-derived schema (`gorm`/`db` tags) can disagree with `.sql` migrations (renamed/dropped columns) → **Mitigation**: migration-derived facts win on conflict, struct-tag facts are kept but flagged in `properties`, build never hard-fails on mismatch.
- **[Risk]** Token-budget accounting is approximate (chars/4), so `context` output could occasionally exceed `maxTokens` for token-dense text → **Mitigation**: bias the estimate conservative (round up) and leave headroom; exact tokenization is a future refinement, not a v1 requirement.
- **[Risk]** Provider-driven summarization (Decision 6) means `summarize pending`/`apply` do nothing on their own — a `build` without a provider running `apply` afterward leaves nodes unsummarized, and `context`/`search` output is weaker (falls back to signature/name only) until summaries exist → **Mitigation**: `context`/`search` degrade gracefully rather than erroring on missing summaries; the CLI should make "N nodes still need summaries" visible (`build` output, `search` results) rather than silent.
- **[Risk]** Lexical search (Decision 7) misses genuinely semantic/topic queries that share no vocabulary with a node's summary (the original design's actual motivation for embeddings) → **Mitigation**: accepted trade-off for a zero-API-key v1; `GetContext`'s primary resolution path (exact name/file match) isn't affected, only the fallback fuzzy path is weaker.
- **[Risk]** Single SQLite file with no daemon means concurrent `build`/`update` runs against the same DB could race → **Mitigation**: v1 is a single-process CLI; document that concurrent invocations against the same repo aren't supported, matching `kv`'s own WAL-and-single-writer posture rather than building a lock manager now.

## Open Questions

- ~~Where does the per-repo SQLite database live by default?~~ Resolved during implementation: an external per-user cache dir keyed by the repo's absolute path (`os.UserCacheDir()/kgraph/<hash>/graph.db`), overridable via `--db`. See `internal/store.DefaultDBPath`.
- ~~Which real Go repository validates the pipeline end-to-end?~~ Resolved: `anti-fraudeiro` (34 files, no vendored deps) is the checkpoint target used for `internal/build`'s integration tests; `kgraph build <repo_path>` remains repo-agnostic by construction.
- Should `kgraph summarize apply` also be reachable as an MCP tool (like `kv mcp`) instead of only a file-based `pending`/`apply` round trip? Not blocking for this MVP — the CLI round trip is sufficient for a provider driving `kgraph` via shell tool calls — but worth revisiting if this becomes a recurring workflow.
