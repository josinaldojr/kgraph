## Why

O kgraph atualmente é uma ferramenta exclusivamente Go: o parser usa `go/ast`, `go/types` e `golang.org/x/tools/go/packages` — ferramentas que só funcionam com código Go. Quando um usuário roda `kgraph build` em um projeto Java, TypeScript, Python ou qualquer outra linguagem, a extração falha ou produz resultados vazios/mínimos. Isso limita drasticamente o público e a utilidade da ferramenta, especialmente considerando que a maioria dos projetos de software modernos são multi-language ou usam linguagens como Java (Spring Boot), TypeScript (NestJS/Express) e Python (FastAPI/Django).

## What Changes

- **Novo sistema de detecção de linguagem**: Identifica automaticamente a linguagem primária do projeto (Go, Java, TypeScript, JavaScript, Python) baseado em arquivos marcadores (`go.mod`, `pom.xml`, `tsconfig.json`, `package.json`, `pyproject.toml`)
- **Interface comum de extração**: Abstrai o parser Go atual atrás de uma interface `Extractor`, permitindo plugins de parser por linguagem
- **Parser Java**: Extrai packages, classes, interfaces, enums, methods, fields, imports, herança (extends/implements), chamadas, annotations Spring/JPA (@Service, @RestController, @Entity, @Table, @Column), e dependências Maven/Gradle
- **Parser TypeScript/JavaScript**: Extrai modules, classes, interfaces, types, functions, imports (ES modules/CommonJS), herança, chamadas, decorators (NestJS, TypeORM), e dependências package.json
- **Parser Python**: Extrai modules, packages, classes, functions, imports, herança, chamadas, decorators (Flask, FastAPI, SQLAlchemy), e dependências requirements.txt/pyproject.toml
- **Novos tipos de nó no grafo**: `Class`, `Enum`, `Decorator`, `Variable`, `TypeAlias` para representar conceitos específicos de outras linguagens
- **Novos tipos de aresta no grafo**: `extends`, `injected`, `decorated`, `routed` para representar relacionamentos específicos de frameworks
- **Suporte a projetos multi-language**: Múltiplos extractores rodando em paralelo, com merge dos grafos resultantes
- **Atualização incremental multi-language**: O pipeline de `kgraph update` roteia arquivos por extensão para o extrator correto

## Capabilities

### New Capabilities
- `language-detection`: Detecção automática da linguagem primária do projeto baseado em arquivos marcadores e extensões de arquivo
- `extractor-interface`: Interface comum `Extractor` que abstrai parsers de linguagem, com factory e roteamento
- `java-extraction`: Extração de grafo de conhecimento de código Java — classes, interfaces, enums, methods, fields, imports, herança, chamadas, annotations Spring/JPA, dependências Maven/Gradle
- `typescript-extraction`: Extração de grafo de conhecimento de código TypeScript/JavaScript — modules, classes, interfaces, types, functions, imports, herança, chamadas, decorators, dependências
- `python-extraction`: Extração de grafo de conhecimento de código Python — modules, packages, classes, functions, imports, herança, chamadas, decorators, dependências
- `multi-language-merge`: Merge de grafos de múltiplos extractores para projetos com mais de uma linguagem

### Modified Capabilities
- `graph-model`: Extensão do modelo de grafo com novos NodeType (Class, Enum, Decorator, Variable, TypeAlias) e EdgeType (extends, injected, decorated, routed)
- `incremental-update`: Extensão do pipeline de atualização incremental para rotear arquivos por extensão de linguagem

## Impact

- **Código afetado**: `internal/parser/` (refatoração completa), `internal/build/` (pipeline de extração), `internal/graph/` (modelo de grafo), `internal/summarizer/` (novos tipos de nó), `internal/server/` (frontend com novos tipos/cores)
- **Novas dependências**: `github.com/smacker/go-tree-sitter` + grammars (`tree-sitter-java`, `tree-sitter-typescript`, `tree-sitter-javascript`, `tree-sitter-python`)
- **Breaking changes**: Nenhum — o comportamento existente para projetos Go é preservado; a interface `Extractor` é interna
- **CGO**: tree-sitter requer CGO (ou binding WASM como alternativa futura)
- **Testes**: Novos fixtures de teste para cada linguagem, testes de detecção, testes de merge multi-language
