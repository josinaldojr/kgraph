## Why

A second AI-assisted review of the archived `kgraph-graphify-evolution` change (following the `graphify-evolution-followups` conformance pass) found ten issues spanning three different natures: two gaps where `server-api` and `rationale-extraction` are silent on behavior the implementation should have (request bounds, panic isolation, multi-line rationale comments); one place where already-accepted spec text (`query-engine`'s literal prompt-section scenario) isn't matched by the code; and seven implementation-only issues (dead code, a stale design doc, a flag that's silently ignored on one code path, thin test coverage, and a low-risk refactor) that need no spec change. Bundling them mirrors the prior follow-up's pattern: close every gap the review surfaced in one pass rather than trickling out single-issue changes.

## What Changes

- **BREAKING (behavioral, not API-shape)**: Add upper bounds to `topK` (`/api/search`), `hops` (`/api/graph/local`, `/api/query`, `/api/explain`), and `max_tokens` (`/api/query`) in `internal/server/api.go`'s `intParam`/topK parsing — today only `n > 0` is checked, so a client can request unbounded work. Out-of-range values SHALL clamp to the max rather than error, per the new `server-api` scenario.
- Add panic-recovery middleware wrapping the server's `http.NewServeMux()` in `internal/server/server.go`, so a handler panic returns a 500 to that request instead of crashing the whole process — significant in hub mode, where one process serves every discovered project.
- Add multi-line continuation to rationale-tag extraction in `internal/enrich/rationale.go`'s `ExtractRationale`: a `NOTE:`/`WHY:`/`HACK:`/`TODO:`/`FIXME:`/`WARNING:` comment whose text continues on immediately-following same-prefix comment lines SHALL have its full text captured, not just the first line.
- Rewrite `internal/context/prompt.go`'s markdown output to match `query-engine`'s already-accepted "Prompt formatted for LLM" scenario exactly: `## Context: <node-id>` (not a bare `# <node-id>` title plus untitled `## Context`), and `### Direct Relations` / `### Related Nodes` as two separate `###` sections (not one merged `## Relations`), with `### Rationale` and `### Community` also at `###`, not `##`. No spec change — the spec already says this; only the implementation is wrong.
- Delete the dead `counts[comm[id]] += 0` line in `internal/analytics/community.go`'s label-propagation loop — verified it can never affect the outcome (real neighbor-community weights are always ≥ 1, so a self-added zero can't win the `>` comparison or the `==` tie-break).
- Correct Decision 4 in the archived `openspec/changes/archive/2026-09-12-kgraph-graphify-evolution/design.md`: it describes "connected-component expansion with modularity-based refinement," but the shipped `DetectCommunities` is label propagation (every node starts in its own community, iteratively adopts its neighbors' plurality community, then merges undersized communities into their best-connected neighbor). Fix the historical record in place rather than leaving it to mislead future readers.
- Fix `cmd/kgraph/cmd_analyze.go`: the documented `--resolution` flag is silently ignored unless `--recluster` is also passed (the non-recluster path calls `analytics.AnalyzeGraph`, which hardcodes `DefaultResolution` internally). Give `AnalyzeGraph` a resolution parameter so both code paths honor the flag.
- Add `internal/enrich/enrich_test.go`: integration-level coverage of `EnrichGraph` itself (file grouping, rationale-to-entity linking end to end via `nearestEntity`), which today only has its two building blocks (`confidence_test.go`, `rationale_test.go`) tested in isolation.
- Add an explicit confidence round-trip assertion to `internal/store/store_test.go`'s `TestSaveAndLoadGraphRoundTrip` (or a new dedicated test): save an edge with `Confidence = graph.ConfidenceInferred` and assert it comes back unchanged after `LoadGraph`. Today the only confidence-touching test covers the old-schema migration default, not a normal round trip.
- Refactor `internal/enrich/enrich.go`'s `nearestEntity` from a linear scan to `sort.Search` over the already-sorted `entities` slice.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `server-api`: new requirement that query-parameter-driven work (topK, hops, max_tokens) is bounded, and that a handler panic doesn't take down the shared process.
- `rationale-extraction`: new requirement that a rationale-tagged comment's text can continue across immediately-following comment lines, not just the tagged line itself.

## Impact

- **Code**: `internal/server/api.go`, `internal/server/server.go`, `internal/enrich/rationale.go`, `internal/enrich/enrich.go`, `internal/context/prompt.go`, `internal/analytics/community.go`, `internal/analytics/analyze.go`, `cmd/kgraph/cmd_analyze.go`.
- **Docs**: `openspec/changes/archive/2026-09-12-kgraph-graphify-evolution/design.md` (Decision 4 correction — historical-record fix, not a behavior change).
- **Tests**: new `internal/enrich/enrich_test.go`; expanded `internal/store/store_test.go`; expanded/added tests in `internal/server`, `internal/enrich/rationale_test.go`, `internal/context/prompt_test.go` (or equivalent) for the bounded params, multi-line continuation, and rewritten prompt sections respectively.
- **Breaking**: out-of-range `topK`/`hops`/`max_tokens` now clamp instead of being silently accepted uncapped — any client relying on very large values will get truncated results instead of an (unbounded, slow) full response. `kgraph prompt`'s markdown output structure changes (headings and section split), which will break any downstream tooling or saved snippets that parsed the old `## Context` / `## Relations` shape.
- **Dependencies**: none new.
