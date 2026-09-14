## Why

`openspec/specs/multi-language-merge/spec.md` already requires that **every** node a language extractor produces carry `Properties["language"]`, but two extractors violate it today, and nothing catches this class of bug: the TypeScript extractor tags npm `ExternalDependency` nodes as `"typescript"` even when it's running in JavaScript mode, and the Go extractor omits `"language"` entirely on `ExternalDependency`, `Table`, and `Column` nodes. Both went unnoticed because no test in the repo asserts the property is set — only `internal/parser/common/merge_test.go` touches `"language"` at all, and it tests merge behavior, not per-extractor output.

## What Changes

- Fix `internal/parser/typescript/parser.go`'s `extractNPMDependencies` (a free function with no access to the extractor) to take the resolved `Language` as a parameter instead of hardcoding `common.LangTypeScript`, so JavaScript-mode npm dependencies are tagged `"javascript"`.
- Add `"language": string(common.LangGo)` to the 5 Go-extractor node-creation sites currently missing it: `ExternalDependency` in `internal/parser/go/packages.go`, and `Table`/`Column` in both `internal/parser/go/orm.go` (struct-tag origin) and `internal/parser/go/migrations.go` (migration origin) — both origins must be fixed together since `graph.AddNode` does last-write-wins on `Properties` with no merge, so a table populated by one origin and later overwritten by the other would otherwise lose the property depending on file-walk order.
- Add a conformance test that extracts a small fixture per language and asserts every resulting node's `Properties["language"]` is set and matches that extractor's `Language()`, so this class of regression is caught going forward instead of relying on manual review.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `multi-language-merge`: the existing "Every extracted node SHALL carry a language property" requirement is generic (one example scenario for a `Java` node). This change adds explicit scenarios for `ExternalDependency`, `Table`, and `Column` nodes — the node types that were actually found non-conformant — so the requirement is unambiguous and directly testable per node type going forward. The normative rule itself (SHALL) is not changing, only its scenario coverage.

## Impact

- `internal/parser/typescript/parser.go` (`extractDependencies`, `extractNPMDependencies`)
- `internal/parser/go/packages.go`, `internal/parser/go/orm.go`, `internal/parser/go/migrations.go`
- New/extended test coverage under `internal/parser/go`, `internal/parser/typescript` (and ideally `internal/parser/java`, `internal/parser/python` for parity)
- No API, storage schema, or CLI surface changes; purely corrects node `Properties` content produced during extraction
