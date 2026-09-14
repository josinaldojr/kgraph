package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectByExtension_TieBreaksDeterministically(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"a.go": "package a\n",
		"b.go": "package b\n",
		"a.py": "def a():\n    pass\n",
		"b.py": "def b():\n    pass\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	d := NewLanguageDetector()
	for i := 0; i < 20; i++ {
		result, err := d.Detect(dir)
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if result.Primary != LangGo {
			t.Fatalf("run %d: got Primary %q, want %q (tied .go/.py counts should resolve to Go every time)", i, result.Primary, LangGo)
		}
	}
}
