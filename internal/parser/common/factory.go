package common

import "fmt"

// ExtractorFactory maintains a registry of language-specific extractors
// and routes to the correct one based on detected language.
type ExtractorFactory struct {
	extractors map[Language]Extractor
}

// NewExtractorFactory creates a new empty ExtractorFactory.
func NewExtractorFactory() *ExtractorFactory {
	return &ExtractorFactory{
		extractors: make(map[Language]Extractor),
	}
}

// Register adds an extractor for the given language to the factory.
// If an extractor for that language already exists, it is replaced.
func (f *ExtractorFactory) Register(lang Language, ext Extractor) {
	f.extractors[lang] = ext
}

// Get returns the extractor for the given language, or an error if none is registered.
func (f *ExtractorFactory) Get(lang Language) (Extractor, error) {
	ext, ok := f.extractors[lang]
	if !ok {
		return nil, fmt.Errorf("factory: no extractor registered for language %q", lang)
	}
	return ext, nil
}

// ForDetection returns the extractors for all languages in the detection result.
// If a language has no registered extractor, it is skipped with a warning.
func (f *ExtractorFactory) ForDetection(result DetectionResult) ([]Extractor, []string) {
	var extractors []Extractor
	var warnings []string

	for _, lang := range result.All {
		ext, ok := f.extractors[lang]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("factory: no extractor for language %q, skipping", lang))
			continue
		}
		extractors = append(extractors, ext)
	}

	return extractors, warnings
}

// Languages returns all registered languages in the factory.
func (f *ExtractorFactory) Languages() []Language {
	langs := make([]Language, 0, len(f.extractors))
	for lang := range f.extractors {
		langs = append(langs, lang)
	}
	return langs
}
