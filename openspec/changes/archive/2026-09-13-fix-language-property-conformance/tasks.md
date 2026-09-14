## 1. TypeScript/JavaScript extractor fix (BUG-1)

- [x] 1.1 Change `extractNPMDependencies` in `internal/parser/typescript/parser.go` to accept a `lang common.Language` parameter instead of hardcoding `common.LangTypeScript`
- [x] 1.2 Update the call site in `extractDependencies` to pass `e.Language()`
- [x] 1.3 Replace both `"language": string(common.LangTypeScript)` literals (dependencies and devDependencies sections, ~lines 740 and 763) with the passed-in `lang`

## 2. Go extractor fix (BUG-2)

- [x] 2.1 Add `"language": string(common.LangGo)` to the `ExternalDependency` node's `Properties` in `internal/parser/go/packages.go` (~line 46-51)
- [x] 2.2 Add `"language": string(common.LangGo)` to the `Table` node's `Properties` in `internal/parser/go/orm.go` (struct-tag origin, ~line 66-68)
- [x] 2.3 Add `"language": string(common.LangGo)` to the `Column` node's `Properties` in `internal/parser/go/orm.go` (struct-tag origin, ~line 85-93)
- [x] 2.4 Add `"language": string(common.LangGo)` to the `Table` node's `Properties` in `internal/parser/go/migrations.go` (migration origin, ~line 142-147)
- [x] 2.5 Add `"language": string(common.LangGo)` to the `Column` node's `Properties` in `internal/parser/go/migrations.go` (`upsertMigrationColumn`, sibling of 2.4)

## 3. Conformance test

- [x] 3.1 Add a table-driven test (in `internal/parser/go`, e.g. alongside existing extractor tests) that extracts the package's existing test fixture and asserts every node's `Properties["language"]` equals `"go"`
- [x] 3.2 Add the equivalent assertion for the TypeScript extractor test suite covering both TypeScript mode (`"typescript"`) and JavaScript mode (`"javascript"`) against an `ExternalDependency` node produced from a `package.json` fixture
- [x] 3.3 Checked `internal/parser/common` — no existing shared fixture/helper for per-extractor test scaffolding (only `merge_test.go`, which tests merge behavior, not extraction). Kept the assertions local to each package's test file rather than introducing a new shared abstraction for two call sites

## 4. Verification

- [x] 4.1 Run `go test ./internal/parser/...` and confirm all pass, including the new conformance tests
- [x] 4.2 Run `go test ./internal/store/... ./internal/context/...` to confirm no test elsewhere asserts exact `Properties` maps for `Table`/`Column`/`ExternalDependency` nodes in a way that breaks from the added field
- [x] 4.3 Run `make vet` and `make fmt` to confirm no formatting/vet regressions
- [x] 4.4 Update this change's `tasks.md` checkboxes as each item completes
