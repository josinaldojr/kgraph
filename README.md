# kgraph

A compact code knowledge graph for AI-assisted code review.

kgraph parses source repositories, extracts a typed graph of code entities and their relationships, and persists it to SQLite. The graph serves as targeted, token-budgeted context for AI code reviewers — never raw source code.

## Features

- **Multi-Language Extraction** — Automatically detects a repository's language(s) (Go, Java, TypeScript, JavaScript, Python) and routes to the matching extractor; multi-language repos have every detected language's subgraph merged into one graph. See [Supported Languages](#supported-languages).
- **Graph Extraction** — Parses Go code using `go/ast` and `golang.org/x/tools/go/packages` to extract packages, structs, interfaces, functions, fields, ORM-mapped tables/columns, SQL migrations, and call/import/embed edges.
- **Incremental Updates** — Reprocesses only files changed since the last build via `kgraph update`.
- **Token-Budgeted Context** — Generates summary-based context for a file, struct, or function, respecting a configurable token budget.
- **Lexical Search** — Searches node summaries and names across the graph.
- **Summarization Pipeline** — Exports pending nodes as JSON for external providers to summarize, then applies the results.
- **Live Visualization** — Serves a browsable HTTP viewer with real-time graph updates.
- **Multi-Project Hub** — Discovers and serves all built graphs from a single hub server.
- **Cache Management** — Lists built projects, detects staleness, and prunes abandoned databases.

## Installation

```bash
go install kgraph/cmd/kgraph@latest
```

Or build from source using the Makefile:

```bash
git clone <repo-url>
cd kgraph
make build          # builds ./bin/kgraph
make install        # builds and installs kgraph to $(go env GOBIN) or $(go env GOPATH)/bin
```

`make install` makes the `kgraph` command available globally, provided that directory is on your `PATH`:

- **macOS/Linux** — add `export PATH="$(go env GOPATH)/bin:$PATH"` to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.).
- **Windows** — add `%USERPROFILE%\go\bin` (or the output of `go env GOPATH` + `\bin`) to your `PATH` via System Properties → Environment Variables, or run `setx PATH "%PATH%;%USERPROFILE%\go\bin"` in a new terminal.

Run `make uninstall` to remove it, or `make help` to see all available targets.

## Usage

### Build a Graph

Extract and persist a repository's knowledge graph:

```bash
kgraph build [repo_path]
```

Without arguments, builds the current directory. The database is stored in a per-user cache directory keyed by the repository path.

```bash
kgraph build /path/to/repo
kgraph build --repo /path/to/repo --db /custom/path/graph.db
```

### Incremental Update

Reprocess only files changed since the last build:

```bash
kgraph update
```

### Generate Context

Print a token-budgeted, summary-based context for a target:

```bash
kgraph context <target>
```

The target can be a file path, struct name, function name, or a search query (used as fallback).

```bash
kgraph context internal/server/api.go
kgraph context Graph
kgraph context ExtractRepo --hops 3 --max-tokens 5000
```

Options:
- `--hops` — How many edges to expand from the target (default: 2)
- `--max-tokens` — Approximate token budget for the rendered context (default: 3000)

### Search

Lexically search node summaries and names:

```bash
kgraph search <query>
```

```bash
kgraph search "database connection"
kgraph search "handler" --top 20
```

Options:
- `--top` — Maximum number of results (default: 10)

### Summarize

Export pending nodes for external summarization:

```bash
kgraph summarize pending --level node --limit 50 > pending.json
```

Apply provider-written summaries:

```bash
kgraph summarize apply pending.json --level node --model gpt-4
```

Levels: `node`, `file`, `module`

### Serve

Serve a live, browsable visualization of the graph over HTTP:

```bash
kgraph serve
```

**Hub mode** (default): Discovers every built graph and serves a project picker at `/`, with per-project viewers under `/p/<key>/`.

**Single-project mode**: Serve a specific repository's graph:

```bash
kgraph serve --repo /path/to/repo
kgraph serve --pick  # interactively choose a built project
```

Options:
- `--addr` — Address to listen on (default: `localhost:7465`)
- `--pick` — Interactively choose a built project to serve

### List Projects

List every built graph database in the cache:

```bash
kgraph projects
```

Shows display name, repository path, last build time, node/edge counts, and freshness relative to git HEAD.

### Prune

Delete abandoned graph databases (repo missing or never built):

```bash
kgraph prune
```

Options:
- `--dry-run` — List candidates without deleting
- `--yes` — Skip confirmation prompt

## Supported Languages

kgraph detects a repository's language(s) from marker files — falling back to counting file extensions if none are found — and runs each detected language's extractor. A repository with more than one language present (e.g. a Java backend plus a TypeScript frontend) has every extractor's output merged into a single graph.

| Language | Marker file(s) | Extractor maturity |
|---|---|---|
| Go | `go.mod` | AST-based (`go/ast`, `go/types`, `golang.org/x/tools/go/packages`) — full fidelity: packages, structs, interfaces, functions, fields, calls, embeds, ORM tables/columns, SQL migrations |
| Java | `pom.xml`, `build.gradle`/`build.gradle.kts` | Regex-based, best-effort — classes, interfaces, enums, methods, fields, imports, inheritance, best-effort calls, Spring annotations (`@Service`/`@RestController`/`@Autowired`/`@GetMapping`/etc.), JPA annotations (`@Entity`/`@Table`/`@Column`), Maven/Gradle dependencies |
| TypeScript / JavaScript | `tsconfig.json` (TS) / `package.json` without `tsconfig.json` (JS) | Regex-based, best-effort — modules, classes, interfaces, type aliases, enums, functions (incl. arrow functions), imports (ES modules/CommonJS), inheritance, best-effort calls, NestJS decorators (`@Controller`/`@Get`/`@Post`/`@Injectable`/`@Inject`), TypeORM decorators (`@Entity`/`@Column`), package.json dependencies |
| Python | `pyproject.toml`, `setup.py`, or `requirements.txt` | Regex-based, best-effort — modules, packages, classes (incl. dataclasses, ABC/Protocol), functions and methods, imports, inheritance, best-effort calls, Flask/FastAPI route decorators (`@app.route`/`@app.get`/`@app.post`), SQLAlchemy declarative models (`__tablename__` + `Column(...)`), requirements.txt/pyproject.toml dependencies |

The Java/TypeScript/JavaScript/Python extractors are regex-based rather than AST-based (a tree-sitter-backed rewrite is planned but not yet wired in — see `openspec/changes/multi-language-support/design.md`), so treat their extraction as best-effort: sound and useful for a graph consumed by an AI reviewer, but lower-fidelity than the Go extractor, especially for cross-file call resolution (no type information is available, so calls and framework-injected dependencies are only linked when a same-named node already exists in the graph — unresolved targets are skipped silently rather than reported as errors).

## Graph Model

### Node Types

| Type | Description |
|------|-------------|
| `Package` | A package or module (Go package, Java package, TS/JS/Python module) |
| `Struct` | A struct or class type |
| `Interface` | An interface type (or a Python ABC/Protocol) |
| `Function` | A function or method |
| `Field` | A struct field / class property |
| `Table` | An ORM-mapped database table |
| `Column` | A table column |
| `Endpoint` | An HTTP endpoint |
| `ExternalDependency` | An imported external package |
| `Enum` | An enum type (Java/TS/Python) |
| `Decorator` | A decorator/annotation (Java annotations, TS decorators, Python decorators) |
| `Variable` | A module-level variable (Python) |
| `TypeAlias` | A type alias (TypeScript) |

### Edge Types

| Type | Description |
|------|-------------|
| `imports` | Package imports another package |
| `calls` | Function calls another function |
| `embeds` | Struct embeds another struct |
| `implements` | Type implements an interface |
| `has_method` | Type has a method |
| `has_field` | Struct has a field |
| `maps_to_table` | Struct maps to a database table |
| `references_fk` | Column references another table via foreign key |
| `reads_table` | Function reads from a table |
| `writes_table` | Function writes to a table |
| `exposes_endpoint` | Function exposes an HTTP endpoint |
| `extends` | Class/interface inheritance (Java/TS/Python) |
| `injected` | Dependency injection (Spring `@Autowired`, NestJS `@Inject`) |
| `decorated` | A class is annotated/decorated (Spring stereotypes, NestJS decorators, Python `@dataclass`) |
| `routed` | A method/function is wired to an HTTP endpoint (Spring, NestJS, Flask, FastAPI) |

## Architecture

```
cmd/kgraph/          CLI commands (cobra)
internal/
  build/             Build pipeline (extraction + persistence)
  context/           Token-budgeted context generation
  gitutil/           Git helpers (commit detection)
  graph/             In-memory graph data structures
  parser/            Multi-language extraction, factory-routed by detected language
    common/          Extractor interface, language detector, factory, graph merge
    go/              Go AST extraction (packages, types, calls, ORM, migrations)
    java/            Java extraction (regex-based, best-effort)
    typescript/      TypeScript/JavaScript extraction (regex-based, best-effort)
    python/          Python extraction (regex-based, best-effort)
  server/            HTTP server, SSE broadcaster, hub, poller
  store/             SQLite persistence (graph, summaries, build metadata)
  summarizer/        Summarization pipeline (pending export, apply, search)
```

## Global Flags

- `--repo` — Path to the repository (default: `.`)
- `--db` — Path to the graph database (default: per-user cache dir keyed by `--repo`)

## License

[Add license here]
