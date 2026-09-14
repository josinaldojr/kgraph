# extractor-interface

## Purpose

Define a interface comum `Extractor` que todos os parsers de linguagem implementam, e o factory que roteia para o extrator correto baseado na linguagem detectada.

## Behavior

### Extractor Interface

Todo parser de linguagem implementa esta interface:

```go
type Extractor interface {
    // ExtractRepo extrai o grafo completo de um repositório.
    ExtractRepo(repoPath string) (*graph.Graph, []string, error)

    // ExtractPackages extrai um subconjunto escopado (para incremental updates).
    // patterns são diretórios/arquivos específicos; knownInternal são pacotes
    // já conhecidos do build anterior.
    ExtractPackages(repoPath string, patterns []string, knownInternal map[string]bool) (*graph.Graph, []string, error)

    // FileExtensions retorna as extensões de arquivo que este extrator processa.
    FileExtensions() []string

    // Language retorna a linguagem que este extrator handle.
    Language() Language
}
```

### ExtractorFactory

O factory mantém um registry de extractors e roteia baseado na linguagem:

```go
type ExtractorFactory struct {
    extractors map[Language]Extractor
}

func NewExtractorFactory() *ExtractorFactory
func (f *ExtractorFactory) Register(lang Language, ext Extractor)
func (f *ExtractorFactory) Get(lang Language) (Extractor, bool)
func (f *ExtractorFactory) ForDetection(result DetectionResult) []Extractor
```

### Integration with Build Pipeline

O pipeline de build é modificado para:

1. Chamar `LanguageDetector.Detect(repoPath)`
2. Chamar `ExtractorFactory.ForDetection(result)` para obter os extractors
3. Para cada extractor, chamar `ExtractRepo(repoPath)`
4. Merge dos grafos resultantes (ver `multi-language-merge`)

O pipeline de update é modificado para:

1. Carregar o grafo existente para determinar as linguagens (via propriedades dos nós)
2. Para cada arquivo mudado, rotear para o extrator correto via extensão
3. Chamar `ExtractPackages()` com os patterns escopados
4. Merge dos subgrafos resultantes

## Edge Cases

- Linguagem não suportada: factory retorna erro claro
- Extrator falha para uma linguagem mas outras sucedem: warnings coletados, grafo parcial retornado
- `ExtractPackages` com patterns vazios: retorna grafo vazio (não erro)

## Acceptance Criteria

- [ ] Interface `Extractor` é definida com todos os métodos necessários
- [ ] `ExtractorFactory` registra e recupera extractors corretamente
- [ ] `ForDetection` retorna os extractors corretos para uma detecção
- [ ] Build pipeline usa factory em vez de chamar parser Go diretamente
- [ ] Update pipeline roteia por extensão de arquivo
- [ ] Erros de um extrator não abortam os outros (best-effort)
- [ ] Warnings de cada extrator são coletados e reportados
