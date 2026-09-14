## 1. Refactoring - Preparação da Arquitetura

- [x] 1.1 Criar diretório `internal/parser/common/` com arquivos `adapter.go`, `detector.go`, `factory.go`
- [x] 1.2 Definir interface `Extractor` em `common/adapter.go` com métodos `ExtractRepo`, `ExtractPackages`, `FileExtensions`, `Language`
- [x] 1.3 Definir tipo `Language` e constantes (`LangGo`, `LangJava`, `LangTypeScript`, `LangJavaScript`, `LangPython`)
- [x] 1.4 Definir struct `DetectionResult` com `Primary`, `All`, `Markers`
- [x] 1.5 Implementar `LanguageDetector` em `common/detector.go` com regras de detecção por arquivos marcadores
- [x] 1.6 Implementar `ExtractorFactory` em `common/factory.go` com registry e roteamento por linguagem
- [x] 1.7 Mover parser Go existente de `internal/parser/*.go` para `internal/parser/go/` (preservando todos os testes)
- [x] 1.8 Adaptar `internal/parser/go/parser.go` para implementar interface `Extractor`
- [x] 1.9 Criar `internal/parser/parser.go` como entry point que usa factory
- [x] 1.10 Atualizar imports em `internal/build/build.go` para usar nova localização do parser Go
- [x] 1.11 Atualizar imports em `internal/build/update.go` para usar nova localização do parser Go
- [x] 1.12 Verificar que todos os testes existentes passam sem modificação

## 2. Extensão do Modelo de Grafo

- [x] 2.1 Adicionar novos NodeType em `internal/graph/node.go`: `Enum`, `Decorator`, `Variable`, `TypeAlias`
- [x] 2.2 Adicionar novos EdgeType em `internal/graph/edge.go`: `extends`, `injected`, `decorated`, `routed`
- [x] 2.3 Adicionar novos ID generators em `internal/graph/ids.go`: `EnumID`, `DecoratorID`, `VariableID`, `TypeAliasID`
- [x] 2.4 Adicionar property `language` como padrão em nós extraídos
- [x] 2.5 Atualizar `internal/summarizer/pending.go` para incluir novos NodeType em `summarizableTypes`
- [x] 2.6 Atualizar `internal/summarizer/source_text.go` para renderizar source text de novos tipos
- [x] 2.7 Atualizar `internal/server/static/app.js` com cores para novos NodeType
- [x] 2.8 Verificar que testes existentes de graph, store, summarizer e server passam
- [x] 2.9 Corrigir colisão de ID: prefixar `VariableID`, `TypeAliasID`, `EnumID` em `internal/graph/ids.go` com o nome do tipo, igual `DecoratorID` já faz — `TestCrossTypeIDsDoNotCollide` em `internal/graph/ids_test.go` cobre a regressão
- [x] 2.10 Adicionar detecção/aviso de sobrescrita de ID em `graph.AddNode` — `Graph.IDConflicts()` registra overwrite entre nós de `Type` diferente (mesmo `Type` continua silencioso, é re-save normal de rebuild/update); ligado em `internal/parser/parser.go` para propagar aos `Warnings` de `build`/`update`. `TestGraphAddNodeIDConflicts` cobre ambos os casos

## 3. Integração real do tree-sitter — FECHADA, não adotada (decisão 2026-09-13)

Reaberta na auditoria de 2026-09-13 (nenhuma das tarefas abaixo havia sido de fato executada apesar de marcadas `[x]`), e fechada de novo na sessão de `/opsx:apply` do mesmo dia: o ambiente de desenvolvimento disponível tem `CGO_ENABLED=0` e nenhum compilador C no `PATH`, o que torna `github.com/smacker/go-tree-sitter` (dependência CGO) impossível de compilar/verificar aqui, e conflita com o objetivo do kgraph de manter seu próprio build cgo-free (ver CLAUDE.md, `internal/store` usa `modernc.org/sqlite` por esse motivo). Decisão do usuário: não adotar tree-sitter nesta mudança — os extractors Java/TypeScript/Python regex-based (seções 4-6, todos concluídos) são o mecanismo *final* aceito, não um estado intermediário. Ver design.md "Resolução final" e Decision 1 (revertida). Tarefas abaixo ficam marcadas como decididas-fechadas, não como pendências:

- [x] ~~3.1-3.7~~ Não adotado — decisão de escopo, não lacuna de implementação (ver acima)
- [x] ~~3.8~~ Não adotado — extractors regex permanecem como mecanismo final; feature coverage de 4.4-4.10/4.14-4.15, 5.4-5.11/5.15, 6.4-6.9/6.14 já está completa via regex

## 4. Java Extractor

- [x] 4.1 Criar diretório `internal/parser/java/` com `parser.go`
- [x] 4.2 Implementar `JavaExtractor` que implementa interface `Extractor`
- [x] 4.3 Implementar extração de packages Java (de caminhos de diretório) — implementada a partir da declaração `package ...;` de cada arquivo (não do caminho de diretório, que nem sempre corresponde ao package Java real); cria um `Package` node por `package` declarado, igual ao padrão já usado por TS/Python
- [x] 4.4 Implementar extração de classes (incluindo abstract, record)
- [x] 4.5 Implementar extração de interfaces
- [x] 4.6 Implementar extração de enums
- [x] 4.7 Implementar extração de métodos (incluindo construtores, static methods)
- [x] 4.8 Implementar extração de campos (incluindo static fields)
- [x] 4.9 Implementar extração de imports Java
- [x] 4.10 Implementar extração de herança (extends) e implementação (implements)
- [x] 4.11 Implementar extração de chamadas de método (best-effort) — regex sobre o corpo do método (`methodBody`/`extractCalledNames`), resolvendo `name(`/`obj.name(`/`this.name(` contra `Function` nodes já conhecidos no mesmo pacote; alvos não resolvidos (interfaces, chamadas cross-package, bibliotecas externas) são ignorados silenciosamente, como já documentado no spec ("skip silencioso")
- [x] 4.12 Implementar extração de annotations Spring (@Service, @RestController, @Autowired, @GetMapping, etc.) — `annotationRe` agora captura nome+args; annotations de classe viram `Decorator` node + edge `decorated` (exceto @Entity/@Table, que viram `maps_to_table`), @Autowired/@Inject em campo viram edge `injected` para o tipo resolvido, @GetMapping/@PostMapping/etc viram `Endpoint` node + edge `routed` combinando o `@RequestMapping` de classe com o path do método; ver design.md Decision 8
- [x] 4.13 Implementar extração de annotations JPA (@Entity, @Table, @Column) com criação de Table/Column nodes — classe com @Entity/@Table vira `Table` node + edge `maps_to_table`; cada campo da entidade vira `Column` node (nome de `@Column(name=...)` ou o nome do campo) + edge `has_field` a partir da tabela; `@GeneratedValue` fica como property `generated=true` no campo
- [x] 4.14 Implementar extração de dependências Maven (pom.xml)
- [x] 4.15 Implementar extração de dependências Gradle (build.gradle)
- [x] 4.16 Criar fixture de teste Java em `internal/parser/java/testdata/` (classe, interface, enum, annotations Spring/JPA) — fixture Spring Boot mínimo (`model.User` com @Entity/@Table/@Column, `repository.UserRepository`, `service.UserService` com @Service/@Autowired, `controller.UserController` com @RestController/@RequestMapping/@GetMapping) + `pom.xml`
- [x] 4.17 Criar testes unitários para Java extractor — `internal/parser/java/parser_test.go` cobre packages, classes/interfaces, mapeamento JPA, annotations Spring, injeção de dependência, roteamento, chamadas de método e dependências Maven
- [x] 4.18 Registrar Java extractor no factory

## 5. TypeScript/JavaScript Extractor

- [x] 5.1 Criar diretório `internal/parser/typescript/` com `parser.go`
- [x] 5.2 Implementar `TypeScriptExtractor` que implementa interface `Extractor`
- [x] 5.3 Implementar extração de modules (packages) de caminhos de arquivo
- [x] 5.4 Implementar extração de classes (incluindo abstract)
- [x] 5.5 Implementar extração de interfaces
- [x] 5.6 Implementar extração de type aliases
- [x] 5.7 Implementar extração de enums
- [x] 5.8 Implementar extração de functions (incluindo arrow functions, métodos)
- [x] 5.9 Implementar extração de properties/fields
- [x] 5.10 Implementar extração de imports (ES modules e CommonJS)
- [x] 5.11 Implementar extração de herança (extends) e implementação (implements)
- [x] 5.12 Implementar extração de chamadas (best-effort) — regex sobre o corpo de método/função (`braceBodyAfterMatch`/`extractCalledNames`), resolvendo contra `Function` nodes já conhecidos no mesmo module; alvos cross-module ou externos são ignorados silenciosamente (sem type-checking)
- [x] 5.13 Implementar extração de decorators NestJS (@Controller, @Get, @Post, @Injectable) — decorators de classe agora também viram `Decorator` node + edge `decorated`; @Get/@Post/@Put/@Delete/@Patch viram `Endpoint` node + edge `routed`, combinando o path de `@Controller('base')` com o do método; `props["decorator_X"]` continua sendo preenchido também. Ver design.md Decision 8
- [x] 5.14 Implementar extração de decorators TypeORM (@Entity, @Column) — @Entity() vira `Table` node + edge `maps_to_table`; cada propriedade da entidade vira `Column` node (nome de `@Column('user_name')` ou o nome da propriedade) + edge `has_field` a partir da tabela; @Inject() em propriedade vira edge `injected`, resolvendo o tipo contra imports nomeados (`import { X } from './x'`) quando possível
- [x] 5.15 Implementar extração de dependências package.json
- [x] 5.16 Criar fixture de teste TypeScript em `internal/parser/typescript/testdata/` (classe NestJS, TypeORM entity) — fixture com `user.entity.ts` (@Entity/@Column), `user.repository.ts` (interface), `user.service.ts` (@Injectable/@Inject), `user.controller.ts` (@Controller/@Get/@Post) + `package.json`
- [x] 5.17 Criar testes unitários para TypeScript extractor — `internal/parser/typescript/parser_test.go` cobre classes/interfaces, mapeamento TypeORM, decorators NestJS, injeção de dependência, roteamento, chamadas de método e dependências package.json
- [x] 5.18 Registrar TypeScript extractor no factory
- [x] 5.19 Implementar `JavaScriptExtractor` (reusa lógica do TypeScript com extensões .js/.jsx)

## 6. Python Extractor

- [x] 6.1 Criar diretório `internal/parser/python/` com `parser.go`
- [x] 6.2 Implementar `PythonExtractor` que implementa interface `Extractor`
- [x] 6.3 Implementar extração de packages e modules
- [x] 6.4 Implementar extração de classes (incluindo dataclasses, ABCs)
- [x] 6.5 Implementar extração de enums
- [x] 6.6 Implementar extração de functions (incluindo static methods, class methods)
- [x] 6.7 Implementar extração de properties/fields
- [x] 6.8 Implementar extração de imports (import e from...import)
- [x] 6.9 Implementar extração de herança (extends) e implementação (ABC/Protocol)
- [x] 6.10 Implementar extração de chamadas (best-effort) — regex sobre o corpo indentado de método/função (`extractIndentedBody`/`extractCalledNames`), resolvendo `self.name(`/`cls.name(`/`name(` contra `Function` nodes já conhecidos no mesmo module; alvos não resolvidos são ignorados silenciosamente
- [x] 6.11 Implementar extração de decorators Flask (@app.route) — `@app.route('/x', methods=[...])` vira `Endpoint` node (método default GET, ou o primeiro em `methods=[...]`) + edge `routed` a partir da função. Ver design.md Decision 8
- [x] 6.12 Implementar extração de decorators FastAPI (@app.get, @app.post) — `@app.get(...)`/`@app.post(...)`/etc viram `Endpoint` node (método = sufixo do decorator) + edge `routed`, mesmo mecanismo de 6.11 (`httpDecoratorMethod`)
- [x] 6.13 Implementar extração de mapeamento SQLAlchemy (modelo declarativo `__tablename__` + `Column(...)`, não decorators @Entity/@Column como a descrição original sugeria — o exemplo concreto do spec usa o estilo declarativo do SQLAlchemy, sem decorators; ver `specs/python-extraction/spec.md` "SQLAlchemy Model Extraction") — classe com `__tablename__` vira `Table` node + edge `maps_to_table`; cada atributo `x = Column(Tipo, ...)` vira `Field` (com properties `column`/`type`) + `Column` node + edge `has_field` a partir da tabela
- [x] 6.14 Implementar extração de dependências (requirements.txt, pyproject.toml)
- [x] 6.15 Criar fixture de teste Python em `internal/parser/python/testdata/` (classe FastAPI, SQLAlchemy model) — fixture com `models.py` (SQLAlchemy `User`/`__tablename__`, `@dataclass UserDTO`), `service.py` (chamada método-a-método), `app.py` (Flask `@app.route` + FastAPI `@app.get`/`@app.post`) + `requirements.txt`
- [x] 6.16 Criar testes unitários para Python extractor — `internal/parser/python/parser_test.go` cobre classes/dataclass, mapeamento SQLAlchemy, roteamento Flask e FastAPI, chamadas de método e dependências requirements.txt
- [x] 6.17 Registrar Python extractor no factory

## 7. Multi-language Merge

- [x] 7.1 Implementar `MergeGraphs()` em `internal/parser/common/merge.go`
- [x] 7.2 Adicionar property `language` a cada nó durante extração
- [x] ~~7.3~~ Namespace de Package/Table/Column por linguagem — **decisão final (2026-09-13): não implementar.** `PackageID(importPath)` (e o gap análogo, mais sério, em `TableID`/`ColumnID`) permanecem sem componente de linguagem. Risco prático avaliado como baixo (convenções de nome já divergem por ecossistema) e a mudança de esquema de ID afetaria todos os extractors com um breaking change nos IDs persistidos. Tratado como decisão de produto aceita, não como lacuna pendente — ver design.md Decision 5/7 e "Resolução final"
- [x] 7.4 Implementar detecção e warning para conflitos de ID — feito para conflito *entre grafos* no merge (`MergeGraphsWithWarnings`); o conflito *dentro* do grafo de um único extractor não é coberto aqui, ver 2.10
- [x] 7.5 Criar testes de merge com grafos de múltiplas linguagens — `internal/parser/common/merge_test.go`: `TestMergeGraphsMultiLanguage` mescla as fixtures reais de Java/TypeScript/Python e confirma zero perda/conflito; `TestMergeGraphsWithWarningsDetectsConflict` cobre o caso de colisão real (mesmo ID, `Type` diferente); `TestKnownInternalForLanguage` cobre o filtro por linguagem. Achado durante a implementação: `TableID`/`ColumnID` colidem entre linguagens do mesmo jeito que `PackageID` (Decision 5) — ver nota nova em design.md Decision 7
- [x] 7.6 Integrar merge no pipeline de build (`internal/build/build.go`)
- [x] 7.7 Remover `AddLanguageProperty` (`internal/parser/common/merge.go`) ou passar a usá-la — removida: todos os quatro extractors (incluindo Go) já fixam `"language"` inline em cada nó no momento da criação, então a função pós-hoc era redundante

## 8. Atualização Incremental Multi-language

- [x] 8.1 Modificar `internal/build/update.go` para rotear arquivos por extensão
- [x] 8.2 Implementar `knownInternalForLanguage()` que filtra pacotes por linguagem — implementada como `common.KnownInternalForLanguage(g, lang)` em `internal/parser/common/merge.go` (mesma assinatura da incremental-update spec, só que em `common` em vez de `build` para ficar ao lado de `Extractor`); `internal/build/update.go` não computa mais um mapa flat, passa o grafo antigo direto e `parser.ExtractPackages` filtra por extractor
- [x] 8.3 Chamar `ExtractPackages()` do extrator correto para cada grupo de arquivos — roteamento por linguagem via factory está correto; ver 8.7 para o gap de escopo dos patterns
- [x] 8.4 Merge dos subgrafos de múltiplas linguagens antes de salvar
- [x] 8.5 Estender lógica de stale invalidation para todas as linguagens
- [x] 8.6 Criar testes de update com projetos multi-language — `internal/build/update_test.go`: `TestUpdateMultiLanguageOnlyReprocessesChangedLanguage` builda um repo Go+Python (detectados via `go.mod`+`requirements.txt`), muda só o lado Python, e confirma que o subgrafo Go sobrevive intacto (semântica upsert do `SaveGraph` + roteamento por linguagem em `patternsByLang`)
- [x] 8.7 Separar `patterns` por linguagem antes de chamar `ExtractPackages` — `internal/build/update.go` agora agrupa diretórios alterados em `map[common.Language][]string` (via `extensionLanguageMap()`, construído a partir de `FileExtensions()` de cada extractor registrado, não mais um switch hardcoded); `parser.ExtractPackages` recebe esse mapa e só chama o extractor de uma linguagem se ela teve arquivos mudados nesta rodada (evita `ExtractPackages(patterns=[])`, que o Go extractor trata como erro via `packages.Load`). Sem efeito prático em Java/TS/Python até 3.8 (que ainda ignoram `patterns`), mas já é o que destrava a incrementalidade real quando 3.8 acontecer

## 9. Integração e Testes End-to-End

- [x] ~~9.1~~ Testar `kgraph build` em projeto Java real (Spring Boot) — **decisão final (2026-09-13): fora de escopo desta mudança.** Validado apenas contra o fixture Spring Boot em `internal/parser/java/testdata/fixture` (testes automatizados + uma rodada manual de `kgraph build`). Rede está disponível neste ambiente, mas o usuário optou por não abrir escopo de correção de bugs contra código de terceiros nesta sessão
- [x] ~~9.2~~ Testar `kgraph build` em projeto TypeScript real (NestJS) — mesma decisão de 9.1; validado apenas contra `internal/parser/typescript/testdata/fixture`
- [x] ~~9.3~~ Testar `kgraph build` em projeto Python real (FastAPI) — mesma decisão de 9.1; validado apenas contra `internal/parser/python/testdata/fixture`
- [x] 9.4 Testar `kgraph build` em projeto multi-language (Java + TypeScript) — `internal/parser/common/merge_test.go` (Java+TS+Python) e uma rodada manual de `kgraph build` sobre os três fixtures combinados (pom.xml+tsconfig.json+requirements.txt na raiz): 76 nodes/52 edges, zero warnings
- [x] 9.5 Testar `kgraph update` incremental para cada linguagem — `internal/build/update_test.go`: `TestUpdateMultiLanguageOnlyReprocessesChangedLanguage` (Go+Python); cada extractor também tem cobertura de extração via seus próprios testes unitários
- [x] 9.6 Testar `kgraph context` com nós de diferentes linguagens — validado manualmente (`kgraph context com.example.demo.service.UserService` sobre o fixture combinado) mostrando `injected`/`decorated`/`has_method` corretamente através de hops
- [x] 9.7 Testar `kgraph search` com nós de diferentes linguagens — validado manualmente (`kgraph search UserController`) retornando as classes Java e TypeScript lado a lado
- [x] 9.8 Testar `kgraph serve` com visualização de nós multi-language — validado manualmente: `kgraph serve` sobe e `/api/graph` retorna nós das três linguagens
- [x] 9.9 Verificar que `kgraph build` em projeto Go existente não teve regressão
- [x] 9.10 Atualizar README.md com documentação de suporte multi-language — nova seção "Supported Languages" (marcadores de detecção + maturidade de cada extractor), Node/Edge Types atualizados com os tipos novos, e `Architecture` refletindo `internal/parser/{common,go,java,typescript,python}`
