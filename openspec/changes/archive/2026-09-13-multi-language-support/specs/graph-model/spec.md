# graph-model (delta)

## Purpose

Extensão do modelo de grafo existente para suportar conceitos de múltiplas linguagens.

## Changes

### New NodeTypes

| NodeType | Descrição | Linguagens |
|---|---|---|
| `Enum` | Tipo enumerado | Java, TypeScript, Python |
| `Decorator` | Annotation/Decorator | Java (@Service), TS (@Injectable), Python (@app.route) |
| `Variable` | Variável module-level | JavaScript, Python |
| `TypeAlias` | Alias de tipo | TypeScript (`type X = ...`) |

### New EdgeTypes

| EdgeType | Descrição | Direção | Linguagens |
|---|---|---|---|
| `extends` | Herança de classe/interface | Child → Parent | Java, TS, Python |
| `injected` | Injeção de dependência | Consumer → Provider | Java (@Autowired), TS (@Inject) |
| `decorated` | Aplicação de decorator/annotation | Entity → Decorator | Java, TS, Python |
| `routed` | Mapeamento de rota HTTP | Handler → Endpoint | Java (@GetMapping), TS (@Get), Python (@app.route) |

### New ID Generators

Each is prefixed with its type name, like `DecoratorID` already was. Go's
`StructID`/`InterfaceID` get away with an unprefixed `importPath.name`
scheme because Go guarantees unique top-level identifiers per package;
Java/TypeScript/Python don't give the same guarantee (e.g. a Python
module-level variable can share a name with a function), so `Enum`,
`Variable` and `TypeAlias` need a discriminator to avoid colliding with
each other or with a same-named `Struct`/`Interface` (audit 2026-09-13;
see design.md Decision 7).

```go
// EnumID returns the ID for an Enum node.
func EnumID(importPath, name string) string {
    return fmt.Sprintf("enum:%s.%s", importPath, name)
}

// DecoratorID returns the ID for a Decorator node.
func DecoratorID(importPath, name string) string {
    return fmt.Sprintf("decorator:%s.%s", importPath, name)
}

// VariableID returns the ID for a Variable node.
func VariableID(importPath, name string) string {
    return fmt.Sprintf("variable:%s.%s", importPath, name)
}

// TypeAliasID returns the ID for a TypeAlias node.
func TypeAliasID(importPath, name string) string {
    return fmt.Sprintf("typealias:%s.%s", importPath, name)
}
```

### Backward Compatibility

- Todos os NodeType e EdgeType existentes são preservados
- Nenhum ID existente muda
- O frontend ignora tipos desconhecidos por padrão (filtro por tipo)
- O SQLite schema não muda (NodeType/EdgeType são strings livres)

## Acceptance Criteria

- [ ] Novos NodeType são definidos como constantes
- [ ] Novos EdgeType são definidos como constantes
- [ ] Novos ID generators são implementados
- [ ] Testes existentes continuam passando sem modificação
- [ ] Frontend não quebra com tipos desconhecidos
