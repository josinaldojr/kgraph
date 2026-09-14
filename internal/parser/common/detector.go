package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// extensionPriorityOrder is the tie-break order for detectByExtension —
// the same fixed priority used for marker detection (Go, Java, TypeScript,
// JavaScript, Python) — so a tie between two languages' file counts
// resolves deterministically instead of depending on map iteration order.
var extensionPriorityOrder = []Language{LangGo, LangJava, LangTypeScript, LangJavaScript, LangPython}

// LanguageDetector detects the primary programming language(s) of a repository
// based on marker files and file extension counting.
type LanguageDetector struct{}

// NewLanguageDetector creates a new LanguageDetector.
func NewLanguageDetector() *LanguageDetector {
	return &LanguageDetector{}
}

// Detect identifies the language(s) of the repository at repoPath.
// It checks for marker files first (go.mod, pom.xml, tsconfig.json, etc.),
// then falls back to counting file extensions if no markers are found.
func (d *LanguageDetector) Detect(repoPath string) (DetectionResult, error) {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return DetectionResult{}, fmt.Errorf("detector: reading %s: %w", repoPath, err)
	}

	// Build a set of root-level file names for marker detection.
	rootFiles := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			rootFiles[e.Name()] = true
		}
	}

	markers := make(map[Language][]string)
	var all []Language

	// Priority 1: go.mod → Go
	if rootFiles["go.mod"] {
		markers[LangGo] = []string{"go.mod"}
		all = append(all, LangGo)
	}

	// Priority 2: pom.xml → Java (Maven)
	if rootFiles["pom.xml"] {
		markers[LangJava] = []string{"pom.xml"}
		all = append(all, LangJava)
	}

	// Priority 3: build.gradle or build.gradle.kts → Java (Gradle)
	if rootFiles["build.gradle"] || rootFiles["build.gradle.kts"] {
		if _, ok := markers[LangJava]; !ok {
			markers[LangJava] = []string{}
			all = append(all, LangJava)
		}
		if rootFiles["build.gradle"] {
			markers[LangJava] = append(markers[LangJava], "build.gradle")
		}
		if rootFiles["build.gradle.kts"] {
			markers[LangJava] = append(markers[LangJava], "build.gradle.kts")
		}
	}

	// Priority 4: tsconfig.json → TypeScript
	if rootFiles["tsconfig.json"] {
		markers[LangTypeScript] = []string{"tsconfig.json"}
		all = append(all, LangTypeScript)
	}

	// Priority 5: package.json (without tsconfig.json) → JavaScript
	if rootFiles["package.json"] {
		if _, hasTS := markers[LangTypeScript]; !hasTS {
			markers[LangJavaScript] = []string{"package.json"}
			all = append(all, LangJavaScript)
		}
	}

	// Priority 6: pyproject.toml, setup.py, or requirements.txt → Python
	var pyMarkers []string
	if rootFiles["pyproject.toml"] {
		pyMarkers = append(pyMarkers, "pyproject.toml")
	}
	if rootFiles["setup.py"] {
		pyMarkers = append(pyMarkers, "setup.py")
	}
	if rootFiles["requirements.txt"] {
		pyMarkers = append(pyMarkers, "requirements.txt")
	}
	if len(pyMarkers) > 0 {
		markers[LangPython] = pyMarkers
		all = append(all, LangPython)
	}

	// If markers found, return result.
	if len(all) > 0 {
		return DetectionResult{
			Primary: all[0],
			All:     all,
			Markers: markers,
		}, nil
	}

	// Fallback: count files by extension.
	return d.detectByExtension(repoPath)
}

// detectByExtension walks the repository and counts files by extension
// to determine the primary language.
func (d *LanguageDetector) detectByExtension(repoPath string) (DetectionResult, error) {
	counts := make(map[Language]int)

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		// Skip hidden directories and vendor/node_modules.
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go":
			counts[LangGo]++
		case ".java":
			counts[LangJava]++
		case ".ts", ".tsx":
			counts[LangTypeScript]++
		case ".js", ".jsx":
			counts[LangJavaScript]++
		case ".py":
			counts[LangPython]++
		}
		return nil
	})
	if err != nil {
		return DetectionResult{}, fmt.Errorf("detector: walking %s: %w", repoPath, err)
	}

	if len(counts) == 0 {
		return DetectionResult{}, fmt.Errorf("detector: no supported source files found in %s", repoPath)
	}

	// Find the language with the most files. Iterate in the same fixed
	// priority order as marker detection (rather than ranging over counts,
	// whose iteration order Go randomizes) so a tie between two languages
	// resolves the same way on every call against the same file tree.
	var primary Language
	var maxCount int
	for _, lang := range extensionPriorityOrder {
		if count := counts[lang]; count > maxCount {
			maxCount = count
			primary = lang
		}
	}

	return DetectionResult{
		Primary: primary,
		All:     []Language{primary},
		Markers: map[Language][]string{primary: {"file-count"}},
	}, nil
}
