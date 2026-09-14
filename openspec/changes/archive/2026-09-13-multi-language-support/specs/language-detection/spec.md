# language-detection

## Purpose

Detecta automaticamente a(s) linguagem(ns) primária(s) de um repositório, baseado em arquivos marcadores e contagem de extensões de arquivo.

## Behavior

### Detection Rules

O detector aplica regras em ordem de prioridade. A primeira regra que match determina a linguagem primária. Se múltiplas regras matcham, o projeto é considerado multi-language.

| Prioridade | Marcador | Linguagem |
|---|---|---|
| 1 | `go.mod` existe | Go |
| 2 | `pom.xml` existe | Java (Maven) |
| 3 | `build.gradle` ou `build.gradle.kts` existe | Java (Gradle) |
| 4 | `tsconfig.json` existe | TypeScript |
| 5 | `package.json` existe (sem tsconfig.json) | JavaScript |
| 6 | `pyproject.toml` ou `setup.py` ou `requirements.txt` existe | Python |

### Fallback

Se nenhum marcador é encontrado, o detector conta arquivos por extensão:
- `.go` → Go
- `.java` → Java
- `.ts`, `.tsx` → TypeScript
- `.js`, `.jsx` → JavaScript
- `.py` → Python

A linguagem com mais arquivos vence. Se nenhuma extensão é encontrada, retorna erro.

### Multi-language

Quando múltiplos marcadores existem (ex: `go.mod` + `package.json`), o detector retorna todas as linguagens encontradas. O caller decide como combinar (ver `multi-language-merge`).

## Interface

```go
type Language string

const (
    LangGo         Language = "go"
    LangJava       Language = "java"
    LangTypeScript Language = "typescript"
    LangJavaScript Language = "javascript"
    LangPython     Language = "python"
)

type DetectionResult struct {
    Primary  Language
    All      []Language
    Markers  map[Language][]string  // e.g. {"java": ["pom.xml"], "typescript": ["tsconfig.json"]}
}

type LanguageDetector interface {
    Detect(repoPath string) (DetectionResult, error)
}
```

## Edge Cases

- Repositório vazio (sem arquivos): retorna erro
- Repositório com apenas `.sql` (migrations): não detecta linguagem de programação
- Marcadores em subdiretórios: ignorados — só a raiz do repositório é verificada
- `package.json` + `tsconfig.json`: TypeScript tem prioridade sobre JavaScript

## Acceptance Criteria

- [ ] Detecta Go corretamente quando `go.mod` existe na raiz
- [ ] Detecta Java corretamente quando `pom.xml` ou `build.gradle` existe
- [ ] Detecta TypeScript corretamente quando `tsconfig.json` existe
- [ ] Detecta JavaScript corretamente quando `package.json` existe (sem tsconfig)
- [ ] Detecta Python corretamente quando `pyproject.toml`, `setup.py` ou `requirements.txt` existe
- [ ] Retorna múltiplas linguagens para projetos multi-language
- [ ] Fallback por contagem de extensões funciona quando nenhum marcador existe
- [ ] Retorna erro para repositórios vazios
