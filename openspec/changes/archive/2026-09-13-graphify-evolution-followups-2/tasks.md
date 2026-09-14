## 1. Server: bounded query parameters

- [x] 1.1 In `internal/server/api.go`, add `MaxHops`, `MaxTopK`, `MaxQueryTokens` constants (proposed: 6, 100, 20000)
- [x] 1.2 Update `intParam` (or add a `clampedIntParam` variant) so values above the relevant max clamp to it instead of passing through uncapped
- [x] 1.3 Apply the clamp to `hops` in `handleGraphLocal`, `handleQuery`, `handleExplain`
- [x] 1.4 Apply the clamp to `topK` in `handleSearch`
- [x] 1.5 Apply the clamp to `max_tokens` in `handleQuery`
- [x] 1.6 Add server tests: oversized `hops`/`topK`/`max_tokens` return a normal 200 response reflecting the clamped value, not an error and not the literal oversized value

## 2. Server: panic recovery

- [x] 2.1 Add a `recoverMiddleware(http.Handler) http.Handler` in `internal/server/server.go` that recovers a panic, logs it (method, path, recovered value), and writes HTTP 500
- [x] 2.2 Wrap the mux with it once at server construction, covering every route including `/events` and the static handler
- [x] 2.3 Add a test that hits a handler forced to panic (e.g. via a test-only route or by injecting a nil dependency) and asserts a 500 is returned and the test process/server keeps serving afterward

## 3. Rationale extraction: multi-line tag continuation

- [x] 3.1 In `internal/enrich/rationale.go`'s `ExtractRationale`, after a line matches `rationaleTagRe`, keep consuming immediately-following lines sharing the same `lineCommentPrefix` that are not themselves a fresh `rationaleTagRe` match, joining their trimmed text onto the rationale's `text` with a space
- [x] 3.2 Stop consuming at a blank line, a non-comment line, end of file, or a line that itself matches `rationaleTagRe`
- [x] 3.3 Update `rationaleNode`'s `LineEnd` to the last consumed continuation line (currently always equal to `LineStart` for tagged comments), so `nearestEntity` and any line-range consumers see the full span
- [x] 3.4 Add test cases in `internal/enrich/rationale_test.go`: a NOTE spanning two lines joins correctly; a NOTE immediately followed by a WHY produces two separate rationale nodes, not one merged one

## 4. Prompt output: match query-engine spec

- [x] 4.1 In `internal/context/prompt.go`, change the title line from `# %s` to `## Context: %s` (node ID), dropping the separate untitled `## Context` header, and fold the existing Type/Signature/Summary bullets under it
- [x] 4.2 Split the current single `## Relations` section into `### Direct Relations` (target's own in/out edges, as today) and `### Related Nodes` (one-hop neighbors reached via those edges, rendered as ID + type + summary, no further edge listing) — see design.md Decision 4 (implemented as hop==2 of a 2-hop `expandSubgraph`, i.e. strictly past Direct Relations, no duplication between the two sections)
- [x] 4.3 Change `## Rationale` to `### Rationale` and `## Community` to `### Community`
- [x] 4.4 Update/create `internal/context/prompt_test.go` asserting the exact heading strings from `query-engine/spec.md`'s "Prompt formatted for LLM" scenario are present, and that the token-budget scenario still truncates correctly with the new section boundaries (added `TestPromptRespectsTokenBudget` in `explain_test.go` alongside the updated `TestPromptRendersMarkdownSections`; also updated `cmd_newcommands_test.go`'s `TestPromptCommandRendersMarkdownSections`)

## 5. Analytics: dead code, stale doc, resolution wiring

- [x] 5.1 Delete the no-op `counts[comm[id]] += 0` line in `internal/analytics/community.go`
- [x] 5.2 Correct Decision 4 in `openspec/changes/archive/2026-09-12-kgraph-graphify-evolution/design.md` to describe label propagation (per-node start, plurality-adopt from neighbors, converge, then `mergeSmallCommunities` folds undersized communities into their best-connected neighbor) instead of connected-component/modularity-refinement
- [x] 5.3 Change `AnalyzeGraph`'s signature in `internal/analytics/analyze.go` to accept `resolution float64` (falling back to `DefaultResolution` when `<= 0`), and pass it to `DetectCommunities`
- [x] 5.4 Update `cmd/kgraph/cmd_analyze.go`'s non-recluster branch to pass the `--resolution` flag's value into `AnalyzeGraph`
- [x] 5.5 Update any other callers of `AnalyzeGraph` (check `internal/build/`) for the new signature — updated `internal/build/build.go` plus test call sites in `community_test.go`, `internal/mcp/tools_test.go`, `internal/export/report_test.go`, `internal/export/json_test.go`
- [x] 5.6 Add/update a test asserting `kgraph analyze --resolution X` (without `--recluster`) actually changes community output versus the default, closing the gap `cmd_newcommands_test.go`'s existing `--recluster --resolution` test doesn't cover — added `TestAnalyzeGraphHonorsResolutionParameter` at the `analytics` package level (a package-level unit test proved more deterministic than a CLI/real-repo fixture for isolating the resolution knob's effect)

## 6. Test coverage: EnrichGraph and confidence round-trip

- [x] 6.1 Add `internal/enrich/enrich_test.go`: a fixture with a code entity and a rationale-tagged comment in the same file, asserting `EnrichGraph` produces the Rationale node, links it via `explains` to the correct entity (using `nearestEntity`'s "next entity at/after, else enclosing" rule), and calls `AnnotateConfidence`
- [x] 6.2 Add a case covering the "source file no longer exists on disk" warning path (non-fatal, returned in the `[]string`, not an error)
- [x] 6.3 Add a confidence round-trip assertion to `internal/store/store_test.go`: save a graph containing an edge with `Confidence = graph.ConfidenceInferred`, reload via `LoadGraph`, assert the confidence value is unchanged (distinct from the existing migration-default test in `schema_test.go`)

## 7. Refactor: nearestEntity binary search

- [x] 7.1 Replace the linear scan in `internal/enrich/enrich.go`'s `nearestEntity` with `sort.Search` over `entities` (already sorted by `LineStart`), preserving the existing "first entity at/after `line`, else last entity before it, else nil" semantics
- [x] 7.2 Confirm existing `enrich`/`rationale` tests (and the new ones from section 6) still pass unchanged, since behavior must be identical — only the search strategy changes

## 8. Verification

- [x] 8.1 `go build ./...`, `go vet ./...`
- [x] 8.2 `go test ./...` passes
- [x] 8.3 Manually run `kgraph serve` against this repo, hit `/api/query?q=auth&hops=999999&max_tokens=999999999` and `/api/search?q=auth&topK=999999`, confirm clamped (not error, not unbounded) responses — confirmed: query returns 200, search returns exactly 100 (`MaxTopK`) results
- [x] 8.4 Manually run `kgraph prompt <some-node>` against this repo and visually confirm the new section headings — confirmed: `## Context: <id>`, `### Direct Relations`, `### Related Nodes`, rationale rendered under a DOCSTRING entry
- [x] 8.5 Manually run `kgraph analyze --resolution 2.0` (no `--recluster`) then `kgraph analyze --resolution 0.5` (no `--recluster`) and confirm the community count actually differs between the two runs — confirmed: 91 vs 69 communities on this repo's own graph
