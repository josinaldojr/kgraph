## Why

A code review comparing the current Java/TypeScript/Python extractors against the already-accepted specs archived under `multi-language-support` found several implementation-conformance gaps and two genuinely missing requirements. The most severe: `ExtractPackages` on all three non-Go extractors ignores `patterns`/`knownInternal` and re-walks the entire repository (`incremental-update` spec explicitly requires scoped extraction "rather than reprocessing the whole repository"), the Python extractor unconditionally creates a `Struct` node before checking for `ABC`/`Protocol` inheritance (violating `python-extraction`'s "an Interface node is created instead of a Struct node" and spamming a spurious ID-conflict warning on every such class), Flask/FastAPI decorator parsing breaks on multi-line decorators because it captures arguments with a single-line, non-balanced-paren regex, and the Java extractor maps a bare `@Table`-without-`@Entity` class to a table when the spec requires `@Entity` to be present. This change closes those gaps and adds the two requirements the review correctly identified as missing (dependency nodes with no `imports` edge into the graph; non-deterministic tie-breaking in extension-count language detection).

## What Changes

- Implement real scoped extraction in `ExtractPackages` for the Java, TypeScript, and Python extractors: filter files by the given directory `patterns` and use `knownInternal` for reference resolution, instead of delegating to `ExtractRepo` on the whole repository.
- Fix the Python extractor's class-handling order so a class inheriting `ABC` or `Protocol` produces only an `Interface` node (no `Struct` node is created first and then overwritten), eliminating the guaranteed spurious `Graph.IDConflicts()` warning this produces today.
- Replace the single-line, non-balanced-paren decorator/annotation-argument capture (`\(([^)]*)\)` and similar `[^)]*`/`[^\]]*` patterns) across the Python, Java, and TypeScript extractors with a small balanced-paren/bracket scanner in `internal/parser/common/`, so multi-line decorators (e.g. a `@app.route(...)` or `@GetMapping(...)` whose arguments span several lines) are captured correctly instead of silently dropped.
- Fix `entityTableName` in the Java extractor so `mapsToTable` requires `@Entity` to be present (matching `java-extraction`'s spec text: "a class is annotated `@Entity` (optionally with `@Table(name = ...)`)"), rather than treating a bare `@Table` as sufficient on its own.
- Fix the Python extractor's `requirements.txt` dependency parsing to populate `Properties["version"]` from the version constraint, as `python-extraction` already requires but the current code never captures.
- **New requirement**: connect each extractor's dependency-manifest-derived `ExternalDependency` nodes (Maven/Gradle for Java, `package.json` for TypeScript/JavaScript, `requirements.txt`/`pyproject.toml` for Python) to the graph with an `imports` edge from the importing `Package`, matching the Go extractor's existing behavior. Today these nodes are created with no incoming or outgoing edge, making them unreachable from any BFS-based context traversal.
- **New requirement**: make `LanguageDetector.detectByExtension`'s majority-language selection deterministic when two or more languages tie on file count (today it depends on Go map iteration order). Ties SHALL be broken by a fixed, defined order.
- Harden `graph.AddEdge` so its `out`/`in` adjacency indices cannot go stale if an edge is ever re-added under the same ID with different `SrcID`/`DstID` than the edge currently stored at that ID (a latent invariant violation, not currently reachable from any extractor or merge path, but undefended).

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `java-extraction`: new requirement that Maven/Gradle-derived `ExternalDependency` nodes are connected to the graph via an `imports` edge from the importing package (previously unconnected).
- `typescript-extraction`: same new `imports`-edge requirement for `package.json`-derived `ExternalDependency` nodes.
- `python-extraction`: same new `imports`-edge requirement for `requirements.txt`/`pyproject.toml`-derived `ExternalDependency` nodes.
- `language-detection`: new requirement that the extension-count fallback breaks ties between languages deterministically instead of relying on map-iteration order.

## Impact

- **Code**: `internal/parser/java/parser.go` (`ExtractPackages`, `entityTableName`, annotation-argument capture), `internal/parser/typescript/parser.go` (`ExtractPackages`, decorator-argument capture, dependency edges), `internal/parser/python/parser.go` (`ExtractPackages`, class/ABC handling order, decorator-argument capture, `requirements.txt` version property, dependency edges), `internal/parser/common/` (new balanced-paren/bracket scanner, shared by all three), `internal/parser/common/detector.go` (`detectByExtension` tie-break), `internal/graph/graph.go` (`AddEdge` index hardening).
- **Tests**: new fixtures/cases per extractor for scoped `ExtractPackages` (pattern filtering, `knownInternal` usage), multi-line decorators/annotations, ABC/Protocol classes (asserting no `IDConflicts()` warning), bare-`@Table`-without-`@Entity` Java classes, dependency `imports` edges, and a forced-tie extension-count fixture.
- **No breaking changes**: all fixes bring behavior into conformance with already-accepted spec text, or add net-new edges/determinism without changing any existing node/edge shape, ID scheme, or CLI surface.
- **Explicitly out of scope** (deferred, per review's P2 bucket): a typed `Language` field on `Node` (conflicts with the graph-model spec's deliberate choice to keep language in `Properties`), reversible ID parsing, a typed `Confidence` enum, and extractor test-coverage-percentage targets (current line coverage for Java/TypeScript/Python is already 77-80%; the real gap is edge-case scenarios, which this change's new tests directly address).
