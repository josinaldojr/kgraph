## Status (auditoria 2026-09-13)

Uma auditoria de código confirmou que este documento e `tasks.md` descreviam como concluído trabalho que não existe no código. Em particular: `github.com/smacker/go-tree-sitter` nunca foi adicionado ao `go.mod` — as Fases 1–4 do Migration Plan abaixo não foram executadas; os extractors Java/TypeScript/Python são 100% regex. A Decision 5 (namespace de Package por linguagem) também não está implementada como descrito. As seções abaixo foram revisadas para refletir o estado real; onde a decisão original permanece válida (ex.: tree-sitter como alvo), o texto foi mantido e o gap fica registrado em `tasks.md`.

## Resolução final (2026-09-13, sessão de apply)

Ao retomar a implementação, o ambiente de desenvolvimento disponível tinha `CGO_ENABLED=0` e nenhum compilador C (`gcc`) no `PATH`. Isso torna a Decision 1 (tree-sitter via CGO) inviável de verificar/compilar aqui, e conflita diretamente com o objetivo do próprio kgraph de manter seu build livre de CGO (`internal/store` usa `modernc.org/sqlite` justamente por isso — ver CLAUDE.md). Com o usuário, decidiu-se:

- **Tree-sitter (Decision 1, tarefas 3.1-3.8): não será implementado nesta mudança.** Os extractors Java/TypeScript/Python regex-based ficam como o mecanismo *final* desta mudança, não como um estado intermediário rumo ao tree-sitter. Ver Migration Plan revisado abaixo.
- **Namespace de Package/Table/Column por linguagem (Decision 5/7, tarefa 7.3): permanece deliberadamente não implementado.** Risco prático avaliado como baixo; tratado como decisão de produto aceita, não como lacuna pendente.
- **Validação contra repositórios OSS reais (tarefas 9.1-9.3): permanece deliberadamente não implementada.** Fixtures cobrem os padrões de framework relevantes (Spring Boot, NestJS, FastAPI/SQLAlchemy); validação contra código real de terceiros fica fora do escopo desta mudança.

Isto fecha o escopo da mudança: as fases regex de Java/TypeScript/Python (com fixtures e testes) são o estado final aceito.

## Context

O kgraph é uma ferramenta de extração de grafo de conhecimento de código para AI-assisted code review. Atualmente, o parser (`internal/parser/`) está 100% acoplado a Go através de `go/ast`, `go/types` e `golang.org/x/tools/go/packages`. O pipeline de build (`internal/build/`) chama diretamente `parser.ExtractRepo()` e `parser.ExtractPackages()`, e o incremental update (`internal/build/update.go`) só reconhece arquivos `.go` e `.sql`.

O modelo de grafo (`internal/graph/`) já é linguagem-agnostic em sua estrutura (Node, Edge, NodeType, EdgeType), mas os tipos de nó existentes refletem conceitos Go: Package, Struct, Interface, Function, Field, Table, Column, Endpoint, ExternalDependency.

O armazenamento (`internal/store/`) e o servidor (`internal/server/`) são completamente linguagem-agnostic — operam sobre o grafo abstrato.

## Goals / Non-Goals

**Goals:**
- Suportar extração de grafo para Java, TypeScript/JavaScript e Python além de Go
- Detectar automaticamente a linguagem do projeto
- Manter 100% de compatibilidade com projetos Go existentes (nenhum breaking change)
- Permitir projetos multi-language (ex: backend Java + frontend TypeScript)
- Suportar incremental update para todas as linguagens suportadas
- Extrair conceitos de framework (Spring, NestJS, Flask, TypeORM, SQLAlchemy)

**Non-Goals:**
- Type-checking completo para linguagens não-Go (tree-sitter não provê isso)
- Resolução de chamadas polimórficas em Java/TypeScript/Python (best-effort, como já é para Go)
- Suporte a linguagens além de Go, Java, TypeScript, JavaScript e Python nesta fase
- Suporte a build systems além de Maven/Gradle (Java), npm/yarn/pnpm (TS/JS), pip/poetry (Python)
- Migração de bancos de dados existentes — novos tipos de nó são adicionais, não substituem

## Decisions

### Decision 1: tree-sitter como base para todos os parsers

**Status final (2026-09-13): revertida.** Ver "Resolução final" no topo do documento — o ambiente de build não suporta CGO de forma confiável e isso conflita com o objetivo cgo-free do projeto. Os extractors Java/TypeScript/Python permanecem regex-based como mecanismo definitivo desta mudança. O texto original da decisão é mantido abaixo por contexto histórico.

**Escolha (original, não adotada):** Usar `github.com/smacker/go-tree-sitter` com as grammars oficiais (`tree-sitter-java`, `tree-sitter-typescript`, `tree-sitter-javascript`, `tree-sitter-python`).

**Alternativas consideradas:**
- **JavaParser (Java)**: Requer JVM rodando — pesado demais para uma CLI Go
- **Regex/Heurístico**: Frágil, perde informações estruturais
- **ANTLR4 + Go**: Complexidade de setup, grammar própria a manter
- **Chamar compiladores nativos** (`javac`, `tsc`): Pesado, requer toolchain instalado

**Rationale:** tree-sitter é rápido (incremental parsing), tem grammars maduras mantidas pela comunidade, funciona como binding Go (CGO), e é consistente entre linguagens. É usado por Neovim, GitHub, Zed e outras ferramentas de análise de código. O trade-off é a dependência de CGO, mas o kgraph já usa `modernc.org/sqlite` que também tem CGO-like overhead.

### Decision 2: Interface Extractor com factory pattern

**Escolha:** Criar uma interface `common.Extractor` e um `common.ExtractorFactory` que roteia para o extrator correto baseado na linguagem detectada.

```
internal/parser/
  common/
    adapter.go      ← interface Extractor
    detector.go     ← LanguageDetector
    factory.go      ← ExtractorFactory
  go/
    parser.go       ← implementa Extractor para Go
    ...
  java/
    parser.go       ← implementa Extractor para Java
    ...
  typescript/
    parser.go       ← implementa Extractor para TS/JS
    ...
  python/
    parser.go       ← implementa Extractor para Python
    ...
```

**Rationale:** Mantém o parser Go existente intacto (apenas movido de diretório), permite adicionar novas linguagens sem modificar código existente, e facilita testes de cada extrator isoladamente.

### Decision 3: Mapeamento de conceitos para o modelo existente

**Escolha:** Reutilizar os NodeType existentes quando o conceito é equivalente, e adicionar novos apenas quando necessário.

| Conceito da linguagem | NodeType kgraph | Rationale |
|---|---|---|
| Java class | `Struct` | Equivalente estrutural (campos + métodos) |
| Java interface | `Interface` | Equivalente direto |
| Java enum | `Enum` (NOVO) | Distingue de struct/class |
| Java/TS method | `Function` | Equivalente direto |
| Java/TS field | `Field` | Equivalente direto |
| TS type alias | `TypeAlias` (NOVO) | Conceito sem equivalente |
| Python class | `Struct` | Equivalente estrutural |
| Python function | `Function` | Equivalente direto |
| Decorator/Annotation | `Decorator` (NOVO) | Conceito transversal |

**Novos EdgeType:**
| Relacionamento | EdgeType | Linguagens |
|---|---|---|
| Herança de classe | `extends` | Java, TS, Python |
| Injeção de dependência | `injected` | Java (@Autowired), TS (@Inject) |
| Decorator/Annotation | `decorated` | Java, TS, Python |
| Rota HTTP | `routed` | Java (@GetMapping), TS (@Get), Python (@app.route) |

### Decision 4: Detecção de linguagem por heurística

**Escolha:** Detectar linguagem por arquivos marcadores e contagem de extensões.

```
Prioridade de detecção:
1. go.mod → Go
2. pom.xml ou build.gradle → Java
3. tsconfig.json → TypeScript
4. package.json (sem tsconfig.json) → JavaScript
5. pyproject.toml ou setup.py ou requirements.txt → Python
6. Fallback: contagem de arquivos por extensão (.go > .java > .ts > .js > .py)
```

Para projetos multi-language, o detector retorna múltiplas linguagens e o factory cria múltiplos extractores.

### Decision 5: Merge de grafos multi-language

**Escolha:** Cada extrator produz um `*graph.Graph` independente. Um `MergeGraphs()`/`MergeGraphsWithWarnings()` combina todos em um único grafo, best-effort com warning em conflito de ID.

**Status real (auditoria 2026-09-13):** implementado como "best-effort com warning", não como "namespace por linguagem". `PackageID(importPath)` não carrega componente de linguagem — na prática isso raramente colide, porque convenções de nome de pacote já divergem entre ecossistemas (`com.example.service` vs `@myapp/core` vs `mypackage` vs `github.com/user/repo`). **O conflito de ID que de fato ocorre e é silencioso é outro**: dentro da mesma linguagem, `StructID`, `InterfaceID`, `EnumID`, `VariableID` e `TypeAliasID` usam o mesmo esquema `importPath.name` — um `Struct Foo` e uma `Variable Foo` no mesmo pacote colidem, e `graph.AddNode` sobrescreve silenciosamente (é um `map[string]*Node`, sem checagem). `MergeGraphsWithWarnings` só detecta conflito *entre grafos* no merge final, não dentro do grafo de um único extrator antes do merge. Ver Decision 7.

**Rationale (original, mantido):** Evita conflitos de ID entre linguagens e mantém o grafo unificado para o servidor e contexto — mas o mecanismo que entrega isso é o discriminador de tipo (Decision 7), não o namespace de Package.

### Decision 6: Atualização incremental multi-language

**Escolha:** O pipeline de `kgraph update` roteia arquivos por extensão para o extrator correto.

```
.go     → GoExtractor.ExtractPackages()
.java   → JavaExtractor.ExtractPackages()
.ts/.tsx→ TypeScriptExtractor.ExtractPackages()
.js/.jsx→ JavaScriptExtractor.ExtractPackages()
.py     → PythonExtractor.ExtractPackages()
.sql    → ExtractMigrations() (existente, linguagem-agnostic)
```

**Rationale:** Reutiliza a lógica de diff/git existente, apenas adicionando o roteamento por extensão.

### Decision 7: Esquema de ID com discriminador de tipo

**Escolha:** Estender a todos os NodeType sem discriminador o mesmo padrão que `DecoratorID` já usa (`"decorator:" + importPath + "." + name`): prefixar `VariableID`, `TypeAliasID` e `EnumID` com o nome do tipo (`"variable:"`, `"typealias:"`, `"enum:"`).

**Por que `StructID`/`InterfaceID` nunca precisaram disso:** em Go, identificadores top-level são únicos por pacote — o compilador impede um `struct Foo` e uma `var Foo` coexistirem. Essa invariante não vale para Python (reatribuição de nome no nível de módulo é válida em tempo de execução) nem, em menor grau, para TypeScript. `Enum`, `Variable` e `TypeAlias` foram adicionados sem essa garantia e herdaram um esquema de ID desenhado para uma linguagem onde o problema não existia.

**Escopo:** isto NÃO resolve o gap de namespace por linguagem descrito na Decision 5 (que se mostrou de baixo risco prático) — resolve a colisão intra-linguagem, que é o bug confirmado.

**Achado relacionado (2026-09-13, ao implementar Decision 8):** `TableID`/`ColumnID` têm o mesmo gap de namespace que `PackageID` (Decision 5), só que com risco prático mais alto: `TableID(tableName)` é só `"table:" + tableName`, sem componente de linguagem nem de módulo/pacote. Ao contrário de nomes de pacote (que já divergem por convenção entre ecossistemas — `com.example.service` vs `mypackage`), nomes de tabela seguem convenções compartilhadas (`users`, `orders`) independentemente da linguagem da aplicação que os acessa. Um teste de merge multi-language real (`internal/parser/common/merge_test.go`) confirmou a colisão: fixtures Java, TypeScript e Python cada uma com uma entidade "User" mapeando para uma tabela `users` colidiam no `table:users` node, mesmo com `Struct` IDs perfeitamente namespaced por pacote/módulo. Isto pode ser o comportamento *desejado* (mesmo banco físico, múltiplos serviços) ou um falso conflito (tabelas não relacionadas que coincidem de nome) — kgraph não tem como distinguir os dois casos hoje. Não resolvido nesta sessão (decisão de produto, não só implementação); os fixtures de teste foram ajustados para nomes de tabela distintos por linguagem para não mascarar isto.

**Trabalho relacionado:** `graph.AddNode` (in `internal/graph/graph.go`) precisa emitir aviso (ou erro) em sobrescrita de ID, não apenas `MergeGraphsWithWarnings` no merge final — ver tasks.md 2.10.

### Decision 8: Extração automática de endpoints/tabelas a partir de annotations e decorators

*(Resolve a Open Question "Endpoint extraction" abaixo — auditoria 2026-09-13 confirmou que isto era esperado mas nunca implementado.)*

**Escolha:** annotations/decorators de framework devem produzir nós de grafo reais, não apenas propriedades soltas no nó que os carrega. `@RestController`/`@GetMapping` (Spring), `@Controller`/`@Get` (NestJS), `@app.route`/`@app.get` (Flask/FastAPI) → `Endpoint` nodes + edge `routed`; `@Entity`/`@Column` (JPA), `@Entity`/`@Column` (TypeORM), SQLAlchemy model classes → `Table`/`Column` nodes + `maps_to_table`; `@Autowired`/`@Inject` → edge `injected`. Isto é o motivo de existirem os EdgeType `routed`/`injected`/`decorated` desde a Decision 3 — hoje nenhum extractor os produz (confirmado: zero ocorrências de `EdgeTypeInjected`/`EdgeTypeDecorated`/`EdgeTypeRouted` em `internal/parser/`).

**Rationale:** sem isso, `Decorator`/`routed`/`injected`/`decorated` são tipos mortos no modelo de grafo — definidos, consumidos por `enrich`/`summarizer`, nunca produzidos.

## Risks / Trade-offs

**[CGO dependency]** tree-sitter requer CGO. → Mitigação: Usar `CGO_ENABLED=1` no build. Alternativa futura: binding WASM via `wazero` se CGO se tornar problemático.

**[Sem type-checking]** tree-sitter provê AST mas não type information. → Mitigação: Call-edge resolution é best-effort (já é assim para Go com interfaces). Análise estática de imports e chamadas diretas cobre 80% dos casos.

**[Performance de parsing]** Múltiplos extractores rodando em paralelo podem ser lentos. → Mitigação: Cada extrator processa apenas arquivos de sua linguagem. tree-sitter é rápido (ms por arquivo).

**[Complexidade de manutenção]** 4 parsers a manter. → Mitigação: Interface comum compartilha lógica de grafo/store/servidor. Cada parser é independente e testável isoladamente.

**[Breaking changes no modelo]** Novos NodeType/EdgeType podem afetar o frontend. → Mitigação: O frontend (`app.js`) já filtra por tipo — novos tipos são ignorados por padrão até serem explicitamente habilitados.

## Migration Plan

Status final por fase (atualizado 2026-09-13, ver "Resolução final" no topo):

1. **Fase 0 (Refactoring):** ✅ Concluída e verificada (`go build`/`go test ./...` passam, parser Go movido com testes intactos).
2. **Fase 1 (Java):** ✅ Concluída — mecanismo final é regex, não tree-sitter (decisão revertida). Extractor Java completo: classes/interfaces/enums/métodos/campos/imports/herança/deps Maven-Gradle, `calls` best-effort, annotations Spring/JPA traduzidas em grafo (Decision 8), fixture e testes.
3. **Fase 2 (TypeScript/JavaScript):** ✅ Concluída — mesmo padrão da Fase 1: extractor regex completo com `calls`, decorators NestJS/TypeORM traduzidos em grafo (Decision 8), fixture e testes.
4. **Fase 3 (Python):** ✅ Concluída — mesmo padrão, adaptado a Python (Flask/FastAPI/SQLAlchemy).
5. **Fase 4 (Multi-language):** ✅ Concluída — `MergeGraphsWithWarnings` integrada ao pipeline, com testes de merge multi-language. O gap de namespace de ID (Decision 5/7) permanece deliberadamente aberto como decisão de produto de baixo risco, não como pendência de implementação.

Tree-sitter (Decision 1) não é adotado nesta mudança — ver "Resolução final".

Rollback: Cada fase é independente. Se uma linguagem específica causar problemas, seu extrator pode ser desabilitado sem afetar as outras.

## Open Questions

- **CGO vs WASM:** Resolvida por decisão de escopo — tree-sitter (CGO ou WASM) não é adotado nesta mudança; ver "Resolução final". Uma futura mudança pode reabrir isto especificamente em torno de um binding WASM (`wazero`) se o time quiser revisitar AST real sem CGO.
- **Novos tipos de nó no frontend:** Como o viewer deve renderizar Class vs Struct? Mesma cor ou diferente?
- **Summarization:** Os prompts de summarização precisam ser diferentes por linguagem? (ex: "this Java class" vs "this Go struct")
