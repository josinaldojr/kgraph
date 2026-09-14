## Context

`internal/parser/common` defines the `Extractor` interface and the `Language` type; each language package (`go`, `java`, `typescript`, `python`) implements it and is responsible for stamping `Properties["language"]` on every node it creates — there is no central place that does this for them. The TypeScript package doubles as the JavaScript extractor (`NewTypeScriptExtractor` vs `NewJavaScriptExtractor`, differentiated by an `isJavaScript bool` field; `Language()` branches on it). `internal/graph.Graph.AddNode` (graph.go:40) replaces a node wholesale by ID with no property merging, so whichever code path adds/overwrites a given ID last determines its final `Properties`.

Two conformance gaps exist against `multi-language-merge`'s existing requirement:
- TypeScript/JavaScript: `extractNPMDependencies` (typescript/parser.go:721) is a **free function**, not a method, so it has no `e *TypeScriptExtractor` in scope and cannot call `e.Language()`. It hardcodes `common.LangTypeScript`.
- Go: `packages.go`, `orm.go`, and `migrations.go` create `ExternalDependency`/`Table`/`Column` nodes without setting `"language"` at all, unlike every other Go node type.

No test in the repo currently asserts the property is present, which is why both gaps shipped unnoticed.

## Goals / Non-Goals

**Goals:**
- Every node any extractor produces carries a correct `"language"` property, closing the two known gaps.
- A regression test exists that would fail if a node type is added/changed without setting `"language"`, so this doesn't silently regress again.

**Non-Goals:**
- Not fixing the `pendingEdge`/`resolvePendingEdges` duplication across java/typescript/python (tracked separately, DRY-1).
- Not making non-Go `ExtractPackages` actually scoped/incremental (tracked separately, PERF-1).
- Not touching `recoverMiddleware`'s empty error body or `ExtractorFactory.Register`'s silent overwrite (STYLE-1/STYLE-2) — unrelated to language tagging.
- Not changing the `Language` type, ID generation, or the merge algorithm itself.

## Decisions

**Thread `Language` into `extractNPMDependencies` as a parameter, rather than making it a method on `*TypeScriptExtractor`.**
The function is a small, pure, already-tested unit (`content string, g *graph.Graph`) called from exactly one place (`extractDependencies`). Adding a `lang common.Language` parameter is the minimal change and keeps the function testable without constructing an extractor. Alternative considered: convert it to a method `(e *TypeScriptExtractor) extractNPMDependencies(...)` — rejected only because it's marginally more churn for no behavioral benefit; either is acceptable and an implementer may choose the method form if it reads better in context.

**Fix both `orm.go` and `migrations.go` Table/Column sites in the same change, not just one.**
Because `graph.AddNode` is last-write-wins on `Properties` with no merge, a `Table` node populated by struct-tag inference and later overwritten by migration inference (or vice versa, depending on `filepath.Walk` order) would keep whichever origin ran last. Fixing only one site would make the bug order-dependent and flaky instead of fixed. Both call sites for `Table` and both for `Column` (struct-tag origin and migration origin) get `"language": string(common.LangGo)` added to their `Properties` map literal.

**Add the conformance test as extractor-level assertions over a small fixture graph, not a change to `graph.AddNode` or a central enforcement point.**
Considered adding a central check (e.g. `Graph.Validate()` that rejects nodes with no `"language"`) but rejected: it would need every call site across all four extractors to already be correct, is a bigger surface change than this bugfix warrants, and conflates a build-time correctness rule with a test-time regression guard. A table-driven test per extractor package (or one shared helper in `internal/parser/common` that each package's test calls with its own fixture + extractor) that walks every node in the resulting graph and asserts `Properties["language"] == extractor.Language()` is narrower, catches exactly this class of bug, and doesn't add runtime cost to extraction itself.

## Risks / Trade-offs

- **[Risk]** Changing `Table`/`Column` `Properties` content could affect `Hash` expectations or any test asserting exact `Properties` maps for those node types. → **Mitigation**: `Hash` is computed from identity strings (e.g. `"struct_tag:" + colID + ...`), not from `Properties`, so hashes are unaffected; run `go test ./internal/parser/go/... ./internal/store/...` after the change to catch any test with a literal `Properties` equality assertion that needs updating.
- **[Risk]** The shared conformance-test approach could tempt scope creep into also fixing DRY-1/PERF-1 while touching the same test files. → **Mitigation**: tasks.md scopes changes explicitly to the 3 files (`packages.go`, `orm.go`, `migrations.go`) and 1 file (`typescript/parser.go`) plus their tests; other findings are out of scope for this change.

## Migration Plan

Not applicable — this is a pure bugfix to extraction logic with no schema, API, or CLI surface change. Existing persisted graphs (SQLite DBs from prior builds) will simply gain the correct `"language"` property on affected node types the next time the repo is rebuilt or those files change and get re-extracted; no migration step is needed since `Table`/`Column`/`ExternalDependency` nodes are content-hashed and re-upserted normally.
