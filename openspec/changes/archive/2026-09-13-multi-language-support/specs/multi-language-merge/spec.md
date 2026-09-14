# multi-language-merge

## Purpose

Combina grafos de múltiplos extractores em um único grafo unificado para projetos com mais de uma linguagem.

## Behavior

### Merge Strategy

Cada extrator produz um `*graph.Graph` independente. O merge combina todos em um único grafo:

```go
func MergeGraphs(graphs ...*graph.Graph) *graph.Graph {
    merged := graph.New()
    for _, g := range graphs {
        for _, n := range g.Nodes() {
            merged.AddNode(n)  // último vence se ID conflita
        }
        for _, e := range g.Edges() {
            _ = merged.AddEdge(e)  // best-effort
        }
    }
    return merged
}
```

### ID Namespacing

IDs são namespaced por caminho de importação/módulo, o que naturalmente evita conflitos entre linguagens:

```
Go:         github.com/user/repo.Service
Java:       com.example.service.UserService
TypeScript: @myapp/users.UserService
Python:     mypackage.users.UserService
```

Conflitos entre linguagens (mesmo ID gerado por dois extractores diferentes) são extremamente raros. O conflito real observado (auditoria 2026-09-13) é intra-linguagem: `Struct`/`Interface`/`Enum`/`Variable`/`TypeAlias` do mesmo extractor podiam compartilhar ID antes da correção do esquema em graph-model (ver Decision 7 do design.md) — `graph.AddNode` agora registra esse caso via `Graph.IDConflicts()` quando dois nós de tipos diferentes reivindicam o mesmo ID.

### Cross-Language Edges

Quando um projeto tem backend Java e frontend TypeScript, podem existir edges cross-language:
- Java REST endpoints → TypeScript fetch calls (não resolvido automaticamente)
- SQL migrations → Java entities e TypeScript types (resolvido via table names)

O merge preserva todos os nós e arestas de cada grafo. Edges cross-language são um Non-Goal desta fase.

### Language Property

Cada nó extraído recebe uma property `language` indicando sua linguagem de origem:

```go
node.Properties["language"] = string(lang)
```

Isso permite filtrar por linguagem no frontend e no contexto.

### Package Node Merging

Quando múltiplas linguagens compartilham o mesmo namespace (ex: um package Java e um module TypeScript com nomes similares), os Package nodes são mantidos separados com IDs distintos.

## Edge Cases

- Projeto com Go backend + TypeScript frontend: grafos independentes, merge simples
- Projeto Java com SQL migrations: merge combina Java graph + migration graph (já funciona)
- Conflito de ID (extremamente raro): último nó vence, warning gerado
- Grafo vazio de uma linguagem: ignorado no merge

## Acceptance Criteria

- [ ] Merge combina nós de múltiplos grafos corretamente
- [ ] Merge combina arestas de múltiplos grafos corretamente
- [ ] Property `language` é adicionada a cada nó
- [ ] IDs não conflitam entre linguagens diferentes
- [ ] Warnings são gerados para conflitos de ID
- [ ] Grafos vazios são ignorados
- [ ] Testes com merge de 2+ linguagens
