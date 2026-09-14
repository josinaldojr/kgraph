## Context

This is the second follow-up pass over the archived `kgraph-graphify-evolution` change (the first was `graphify-evolution-followups`, which brought `graph.json` export into conformance with its spec and added missing `internal/graph`/`internal/enrich` test coverage). A second AI-assisted review of the same code surfaced ten more issues, verified against the current code in this session:

- `internal/server/api.go`'s `intParam` (used for `hops`, `max_tokens`) and the inline `topK` parsing in `handleSearch` only check `n > 0` — no ceiling.
- `internal/server/server.go` wires a bare `http.NewServeMux()` with no panic-recovery wrapper anywhere in the package. `kgraph serve`'s hub mode (`internal/server/hub.go`) runs every discovered project's API off one process, so a panic in one project's handler currently takes the whole hub down.
- `internal/enrich/rationale.go`'s `ExtractRationale` matches a rationale tag against a single line (`strings.Split(content, "\n")`, one iteration per line) — a tag whose text wraps onto the next comment line is truncated at the first line.
- `internal/context/prompt.go` doesn't match `query-engine`'s already-accepted "Prompt formatted for LLM" scenario: it emits `# <id>` / `## Context` / `## Relations` (merged) / `## Rationale` / `## Community`, where the spec requires `## Context: <id>` / `### Direct Relations` / `### Related Nodes` (split) / `### Rationale` / `### Community`.
- `internal/analytics/community.go:66`'s `counts[comm[id]] += 0` is dead: real neighbor-community weights are always ≥ 1 (they come from incrementing edge counts), so a self-added zero-weight candidate can never win the `>` comparison or the `== bestCount` tie-break against any real candidate.
- The archived `2026-09-12-kgraph-graphify-evolution/design.md`'s Decision 4 describes "connected-component expansion with modularity-based refinement," but the shipped `DetectCommunities` is label propagation (per-node community, plurality-adopt from neighbors, converge, then fold undersized communities into their best-connected neighbor via `mergeSmallCommunities`).
- `cmd/kgraph/cmd_analyze.go`'s `--resolution` flag is read only inside the `--recluster` branch; plain `kgraph analyze --resolution X` calls `analytics.AnalyzeGraph`, which hardcodes `DefaultResolution` and ignores the flag entirely.
- `internal/enrich/enrich_test.go` doesn't exist — `EnrichGraph` (the orchestrator: grouping nodes by file, extracting rationale, linking via `nearestEntity`) is untested as a unit, even though its two building blocks are each tested separately.
- `internal/store/store_test.go`'s round-trip test never asserts on `Edge.Confidence`; the only confidence-touching test covers the old-schema migration default.
- `internal/enrich/enrich.go`'s `nearestEntity` does a linear scan over an already-sorted slice.

## Goals / Non-Goals

**Goals:**
- Close all ten review findings in one change, matching the bundling pattern of `graphify-evolution-followups`.
- Add the two genuinely-missing behaviors (bounded server params, panic isolation) as real spec requirements under `server-api`, and the missing multi-line rationale behavior under `rationale-extraction`, rather than treating them as silent implementation details.
- Bring `internal/context/prompt.go` into conformance with `query-engine`'s existing, unchanged spec text.
- Fix the two implementation bugs (dead code, ignored `--resolution` flag) and the one stale doc (Decision 4).
- Add the two missing test-coverage areas (`EnrichGraph`, confidence round-trip).
- Apply the `nearestEntity` binary-search refactor, accepting it's a minor win on typical per-file entity counts — included because it's low-risk and was explicitly flagged by the review, not because it's expected to matter in practice.

**Non-Goals:**
- No change to `query`/`path`/`explain`/`analyze`/`export`/`report`/`mcp` behavior beyond what's listed above.
- No new analytics algorithm (label propagation itself is unchanged — only its description in the archived doc and the ignored resolution flag are fixed).
- No general request-rate-limiting or auth story for the server — this only bounds the three specific query parameters already in the API.
- No retroactive correction of other historical decisions in the archived change beyond Decision 4.

## Decisions

### Decision 1: Clamp out-of-range params instead of rejecting them

**Choice**: `hops`, `topK`, and `max_tokens` clamp silently to a fixed maximum (proposed: `MaxHops = 6`, `MaxTopK = 100`, `MaxQueryTokens = 20000`) rather than returning a 400.

**Rationale**: These are already forgiving endpoints (`intParam` falls back to a default on a non-numeric or non-positive value instead of erroring); clamping is consistent with that existing "be permissive" posture and keeps every existing caller working, just bounded, instead of introducing a new error path clients would need to handle. Also fixes the current silent-fallback asymmetry: today a negative or non-numeric value silently falls back to the default, but an oversized positive value is accepted uncapped — clamping makes both directions behave the same way (never trust the raw client value past a safe bound).

**Alternatives considered**:
- Reject with 400 on out-of-range values: more correct in a REST-purist sense, but inconsistent with how this API already treats bad input (silent fallback, not error), and would break any existing caller passing a large-but-well-intentioned value.
- Make the max configurable via server flag/env var: unnecessary complexity — these are safety ceilings, not tunables anyone has asked to adjust; a Go constant is enough, matching how `DefaultHops`/`DefaultMaxTokens` are already plain constants in `internal/context/context.go`.

### Decision 2: Panic recovery as `net/http` middleware wrapping the mux, not per-handler

**Choice**: Wrap the existing `http.NewServeMux()` once, in `internal/server/server.go`, with a `recover()`-based middleware that logs the panic (with request method/path) and writes a 500, rather than adding `recover()` inside each of the seven handler functions.

**Rationale**: A single wrap point covers every current and future handler and can't be forgotten on a new endpoint the way a per-handler `defer recover()` could be. This is the standard Go pattern for this problem and requires no new dependency.

**Alternatives considered**:
- `recover()` inside each handler: rejected — duplicated boilerplate, easy to miss on a new handler, and doesn't protect `handleEvents`'s SSE loop or `singleIndexHandler()` unless remembered separately.
- A third-party middleware library: rejected — one small wrapper function is simpler than a new dependency for something this contained.

### Decision 3: Multi-line rationale continuation is prefix-and-indentation based, not blank-line-terminated prose reflow

**Choice**: After a line matching `rationaleTagRe`, keep consuming immediately-following lines that (a) start with the same line-comment prefix and (b) are not themselves a fresh recognized tag, appending their trimmed text (space-joined) to the rationale's text, until a line that doesn't meet both conditions.

**Rationale**: Mirrors the existing `extractGoDocstrings` continuation logic (a contiguous run of `//` lines) already in the same file, so the two extraction paths behave consistently. Stopping at a fresh tag prevents a `// NOTE: ...` immediately followed by `// WHY: ...` from being merged into one rationale.

**Alternatives considered**:
- Terminate only on blank line or end-of-comment-block (allow a fresh tag to be swallowed into the previous one's text): rejected — would make two independently-meaningful tags read as one, worse than today's truncation.
- Require an explicit continuation marker (e.g. trailing `\`): rejected — no existing convention in this codebase uses that, and the spec scenario (see `specs/rationale-extraction/spec.md`) describes plain wrapped comment lines.

### Decision 4: `prompt.go` rewrite splits "Direct Relations" from "Related Nodes"

**Choice**: The spec's two relation sections map to: **Direct Relations** = edges directly touching the target node (what `prompt.go` today calls `## Relations`, built from `g.OutEdges`/`g.InEdges` on the target); **Related Nodes** = one level further out — nodes reachable via those direct relations, rendered with just their ID/type/summary (no further edges), giving the LLM lightweight awareness of the target's neighborhood without a full second BFS hop's edge list.

**Rationale**: `query-engine`'s scenario names both sections but doesn't define "Related Nodes" precisely; this reading keeps `Prompt` reusing the same `internal/context` primitives (`explain.go`'s pattern of target-edges plus one-hop neighbor summaries) rather than inventing a new traversal, consistent with `query-engine`'s "Query commands SHALL reuse existing context engine" requirement.

**Alternatives considered**:
- "Related Nodes" = same BFS-expanded set `Explain`/`Query` already collect at `hops` depth: considered, but `Prompt` doesn't currently take a `hops` parameter and adding one would be new API surface beyond what the spec's literal scenario asks for; deferred unless the token-budget scenario turns out to need it.

### Decision 5: `AnalyzeGraph` takes `resolution float64` as a parameter

**Choice**: Change `AnalyzeGraph(g *graph.Graph, godNodeCount int)` to `AnalyzeGraph(g *graph.Graph, godNodeCount int, resolution float64)`, with `resolution <= 0` falling back to `DefaultResolution` (matching `DetectCommunities`'s own existing fallback), and update `cmd_analyze.go`'s non-recluster branch to pass the flag's value through instead of relying on the hardcoded default.

**Rationale**: Smallest change that makes the already-documented `--resolution` flag actually apply on every code path, matching the parameter shape `DetectCommunities` already uses.

### Decision 6: Correct the archived design.md in place

**Choice**: Edit Decision 4 of `openspec/changes/archive/2026-09-12-kgraph-graphify-evolution/design.md` directly to describe label propagation + resolution-based merging, rather than adding a correction note elsewhere.

**Rationale**: User confirmed this in discovery — an archived change's design doc is meant to be an accurate historical record of what was actually decided and shipped, not a preserved snapshot of a description that turned out to be wrong before it was even merged. Leaving it stale would keep misleading anyone who reads that decision to understand why communities are computed the way they are.

## Risks / Trade-offs

**[Risk] Clamping `topK`/`hops`/`max_tokens` changes existing response shape for any caller already sending oversized values**
→ Called out as breaking in the proposal. Mitigation: none needed beyond documentation — no external API consumers are known, and the alternative (staying unbounded) is the actual problem this change fixes.

**[Risk] `prompt.go`'s rewritten section structure breaks anything that parsed the old markdown**
→ Same as above — breaking, documented in the proposal. `kgraph prompt`'s only stated purpose is "paste into an LLM conversation," so no machine-readable-format guarantee is being broken, but any saved snippets or scripts grepping for `## Relations` will need updating.
→ Add/update `internal/context/prompt_test.go` (or equivalent, if none exists yet) asserting the exact heading text from the spec scenario, so a future regression is caught the same way this one wasn't.

**[Risk] Panic-recovery middleware could mask a bug that should be visible**
→ Mitigation: log the panic (stack trace) server-side before returning 500, so it's still observable in server logs — recovery isolates the blast radius, it doesn't hide the failure.

**[Risk] Continuation-line heuristic for rationale (Decision 3) could over-merge in languages/styles not yet exercised by tests**
→ Mitigation: scope the new spec requirement and its scenarios to the same prefix-detection logic `lineCommentPrefix` already uses (`//` or `#`), and add the "stops at blank/fresh-tag" scenario as an explicit test case, not just the happy path.

## Open Questions

None outstanding — the two ambiguous points from discovery (whether to include the `nearestEntity` binary-search refactor, and where to correct the stale Decision 4) were resolved with the user before writing this design: include the refactor, and edit the archived design.md directly.
