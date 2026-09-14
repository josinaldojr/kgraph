## 1. Shared balanced-paren/bracket scanner

- [x] 1.1 Add `common.ScanBalanced` (or equivalent) to `internal/parser/common/` — scans forward from an opening delimiter to its matching close, tracking nesting depth and skipping delimiters inside quoted strings; operates on the whole remaining text, not a single line.
- [x] 1.2 Unit-test the scanner directly: nested brackets, unbalanced/missing close, delimiters inside single- and double-quoted strings, multi-line input.

## 2. Python extractor fixes

- [x] 2.1 Reorder `extractFile`'s class handling so the ABC/Protocol check runs before any node is created, and exactly one node (`Interface` or `Struct`) is added per class — no more unconditional `Struct` creation followed by an overwriting `Interface` add.
- [x] 2.2 Add a fixture case (a class inheriting `ABC` or `Protocol`) and assert: the resulting node's `Type` is `Interface`, no `Struct` node exists at that ID, and `Graph.IDConflicts()` reports nothing for it.
- [x] 2.3 Replace `parseDecorators`'s line-by-line, single-line-paren decorator capture with the Decision 1 scanner so multi-line decorator arguments are captured.
- [x] 2.4 Add a fixture case for a multi-line `@app.route(...)` (or FastAPI equivalent) with `methods=[...]` spanning multiple lines; assert the correct HTTP method and path are extracted.
- [x] 2.5 Fix `extractRequirementsDependencies` to populate `Properties["version"]` from the version constraint in `requirements.txt`, per the existing `python-extraction` spec requirement.
- [x] 2.6 Add a fixture/assertion for the `version` property (e.g. `flask>=2.0.0` → `Properties["version"] == ">=2.0.0"`).
- [x] 2.7 Implement real scoped extraction in `ExtractPackages`: filter the file walk to only files whose directory (relative to `repoPath`, slash-normalized) is in `patterns`; return an empty graph for empty `patterns`.
- [x] 2.8 Classify each `import`/`from ... import` target as internal (resolves to a `Package` node produced by this walk, or present in `knownInternal`) or external; for external targets, create the `imports` edge to `graph.ExternalDependencyID(...)` (reusing a manifest-derived node when the name matches, creating one tagged `language="python"` otherwise) instead of a bare `Package` stub. (Required fixing two adjacent pre-existing bugs to work correctly: relative imports (`from .foo import X`) weren't resolved against the importing module's package at all, and `pyFromImportRe`'s trailing char class used `\s` instead of `[ \t]`, so it greedily consumed into the next line's `from` statement whenever two from-imports appeared back to back.)
- [x] 2.9 Add fixture cases: an external import (e.g. `import flask`) produces an `imports` edge to `ext:flask` and no bare `pkg:flask` node; an internal import still produces a `Package`-to-`Package` edge; a scoped `ExtractPackages` call with `patterns` limited to one directory only extracts files under that directory.

## 3. Java extractor fixes

- [x] 3.1 Fix `entityTableName` so `mapsToTable` requires `hasEntity` (bare `@Table` without `@Entity` no longer maps to a table), per the existing `java-extraction` spec text. (Also had to stop passing `tableName` into `extractFields` when `mapsToTable` is false — it was still populated from `@Table`'s `name=` arg regardless, which was producing a `has_field` edge from a `Table` node that no longer gets created.)
- [x] 3.2 Add a fixture case: a class annotated only `@Table(name = "...")` with no `@Entity` produces no `maps_to_table` edge and no `Table` node from that class.
- [x] 3.3 Replace annotation-argument capture (`@(\w+)(?:\(([^)]*)\))?` and similar) with the Decision 1 scanner so multi-line annotations (e.g. a `@RequestMapping(...)` whose arguments span multiple lines) are captured. (Mirrors the Python fix's approach exactly: span-scan the whole file/class-body text once, then find the contiguous run of spans immediately preceding a declaration.)
- [x] 3.4 Add a fixture case for a multi-line Spring mapping annotation; assert the resulting `Endpoint`/`routed` edge is still created correctly.
- [x] 3.5 Implement real scoped extraction in `ExtractPackages`: same directory-filtering approach as task 2.7.
- [x] 3.6 Classify each Java `import` target as internal/external (same approach as task 2.8); external targets get an `imports` edge to `ExternalDependencyID`, reusing/creating a node tagged `language="java"`. (Java's manifest deps are keyed `groupId:artifactId`, which has no reliable mapping from a Java import path, so the external ID uses a coarse 2-component package-root heuristic, e.g. `org.springframework` — matches this change's `java-extraction` spec delta.)
- [x] 3.7 Add fixture cases mirroring task 2.9 for Java (external import → `imports` edge to `ExternalDependency`; scoped `ExtractPackages` limited to one package directory).

## 4. TypeScript/JavaScript extractor fixes

- [x] 4.1 Replace decorator-argument capture with the Decision 1 scanner so multi-line decorators (e.g. NestJS `@Get(...)` spanning multiple lines) are captured.
- [x] 4.2 Add a fixture case for a multi-line decorator; assert the resulting edge (`routed`/`decorated`/etc., whichever the fixture's decorator implies) is still created correctly.
- [x] 4.3 Implement real scoped extraction in `ExtractPackages`: same directory-filtering approach as task 2.7.
- [x] 4.4 Classify each `import`/`require` target as internal/external (same approach as task 2.8); external targets get an `imports` edge to `ExternalDependencyID`, reusing/creating a node tagged with the extractor's actual running language (`typescript` or `javascript` — preserve the existing per-mode tagging fixed by `fix-language-property-conformance`). (Also fixes a second, adjacent bug: the old code created `imports` edges straight from the raw, unresolved specifier — e.g. `pkg:./service` — for relative imports too, which never matched the real `pkg:<dotted.module>` scheme; it now runs relative specifiers through the existing `resolveImportModule` first, same as the named-import map already did for extends/implements/inject resolution.)
- [x] 4.5 Add fixture cases mirroring task 2.9 for TypeScript and JavaScript modes (external import → `imports` edge to `ExternalDependency`; relative import stays internal; scoped `ExtractPackages` limited to one directory).

## 5. Language detection determinism

- [x] 5.1 Change `detectByExtension`'s tie-breaking to iterate languages in the fixed priority order (Go, Java, TypeScript, JavaScript, Python) and keep the first language reaching the highest count, instead of iterating the `counts` map directly.
- [x] 5.2 Add a test fixture with equal `.go` and `.py` file counts and no marker files; assert `Detect` deterministically returns Go, across repeated calls.

## 6. Graph adjacency-index hardening

- [x] 6.1 In `graph.AddEdge`, when `e.ID` already exists, compare the existing edge's `SrcID`/`DstID` to the incoming edge's; if they differ, remove the stale entries from `out[oldSrcID]`/`in[oldDstID]` and add the new ones to `out[e.SrcID]`/`in[e.DstID]`.
- [x] 6.2 Add a unit test in `internal/graph/` that adds an edge, then adds a second edge with the same `ID` but different `SrcID`/`DstID`, and asserts `OutEdges`/`InEdges` reflect only the current endpoints (no stale entries under the old endpoints).

## 7. Full verification

- [x] 7.1 `go build ./...`, `go vet ./...`, `go fmt ./...` clean.
- [x] 7.2 `go test ./...` passes, including all new fixtures/cases above.
- [x] 7.3 Manually ran `kgraph build`/`export`/`update` (real CLI binary, not just unit tests) against a scratch Python fixture (Flask multi-line route decorator, an `ABC`-derived class, `flask`/stdlib `abc` imports, `requirements.txt`). Confirmed via `graph.json`: `Repository` is `Interface`-typed with no `Struct` duplicate and no `IDConflicts` warning printed; `ext:flask` carries `version: ">=2.0.0"` and has an incoming `imports` edge from `pkg:myapp.app`; `ext:abc` (stdlib) got an `imports` edge too, created on the fly; the multi-line `@app.route(...)` produced the correct `GET /users` endpoint and `routed` edge. Then committed the fixture to a throwaway git repo and ran `kgraph update` after touching the one file — it went through the scoped `ExtractPackages` path cleanly ("1 file(s) changed, 5 node writes, 4 edge writes"), no errors or warnings.
