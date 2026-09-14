# incremental-update (delta)

## Purpose

Extensão do pipeline de atualização incremental para suportar múltiplas linguagens, roteando arquivos por extensão para o extrator correto.

## Changes

### File Routing

O update pipeline é modificado para rotear arquivos mudados por extensão:

```
Arquivo mudado → Extensão → Extrator
─────────────────────────────────────
*.go            → GoExtractor.ExtractPackages()
*.java          → JavaExtractor.ExtractPackages()
*.ts, *.tsx     → TypeScriptExtractor.ExtractPackages()
*.js, *.jsx     → JavaScriptExtractor.ExtractPackages()
*.py            → PythonExtractor.ExtractPackages()
*.sql           → ExtractMigrations() (existente)
outros          → ignorado (warning)
```

### Scoped Extraction

Para cada linguagem, os arquivos mudados são agrupados por diretório/package:

```
Java:
  src/main/java/com/example/service/UserService.java
  src/main/java/com/example/service/OrderService.java
  → patterns: ["src/main/java/com/example/service"]

TypeScript:
  src/users/user.controller.ts
  src/users/user.service.ts
  → patterns: ["src/users"]
```

O extrator correspondente é chamado com `ExtractPackages(repoPath, patterns, knownInternal)`.

### Known Internal Packages

O `knownInternal` é construído a partir do grafo existente, filtrando por linguagem:

```go
func knownInternalForLanguage(g *graph.Graph, lang Language) map[string]bool {
    out := make(map[string]bool)
    for _, n := range g.Nodes() {
        if n.Type == graph.NodeTypePackage {
            if langProp, _ := n.Properties["language"].(string); langProp == string(lang) {
                out[strings.TrimPrefix(n.ID, "pkg:")] = true
            }
        }
    }
    return out
}
```

### Merge

Os subgrafos extraídos por cada linguagem são merged em um único grafo antes de salvar (ver `multi-language-merge`).

### Stale Invalidation

A lógica de invalidação de summaries stale é estendida para considerar nós de todas as linguagens. A lógica existente (vizinhos diretos de nós mudados) funciona independentemente da linguagem.

## Edge Cases

- Arquivo renomeado (git mv): detectado como delete + add, funciona corretamente
- Arquivo movido entre pacotes: nó antigo deletado, nó novo criado
- Linguagem adicionada ao projeto (novo `pom.xml`): na próxima build completa, o novo extrator é ativado
- Linguagem removida do projeto: nós existentes permanecem até próxima build completa

## Acceptance Criteria

- [ ] Arquivos .java são roteados para JavaExtractor
- [ ] Arquivos .ts/.tsx são roteados para TypeScriptExtractor
- [ ] Arquivos .js/.jsx são roteados para JavaScriptExtractor
- [ ] Arquivos .py são roteados para PythonExtractor
- [ ] Arquivos .go continuam roteados para GoExtractor (sem regressão)
- [ ] Arquivos .sql continuam processados por ExtractMigrations (sem regressão)
- [x] knownInternal é filtrado por linguagem (`common.KnownInternalForLanguage`, ligado em `internal/parser/parser.go`)
- [ ] Subgrafos de múltiplas linguagens são merged corretamente
- [ ] Stale invalidation funciona para nós de todas as linguagens
- [ ] Testes de update com projetos multi-language
