package viewer

import (
	"bytes"
	"strings"
	"testing"
)

func TestAssetsServeCSSAndJS(t *testing.T) {
	fsys := Assets()
	for _, name := range []string{"app.css", "app.js"} {
		f, err := fsys.Open(name)
		if err != nil {
			t.Fatalf("Assets().Open(%q) error = %v", name, err)
		}
		f.Close()
	}
}

func TestCSSAndJSReturnNonEmptyContent(t *testing.T) {
	if len(CSS()) == 0 {
		t.Error("expected CSS() to return non-empty content")
	}
	if len(JS()) == 0 {
		t.Error("expected JS() to return non-empty content")
	}
}

func TestRenderIndexInjectsBootstrapConfig(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{Mode: "single", Page: "viewer", APIBase: "/api", EventsPath: "/events"}
	if err := RenderIndex(&buf, cfg); err != nil {
		t.Fatalf("RenderIndex() error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"mode":"single"`) {
		t.Errorf("expected rendered page to embed bootstrap config, got:\n%s", out)
	}
	if !strings.Contains(out, `window.KGRAPH`) {
		t.Errorf("expected rendered page to assign window.KGRAPH, got:\n%s", out)
	}
}

func TestRenderIndexPickerPageOmitsCanvas(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{Mode: "hub", Page: "picker", APIBase: "/api"}
	if err := RenderIndex(&buf, cfg); err != nil {
		t.Fatalf("RenderIndex() error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `id="picker"`) {
		t.Errorf("expected picker page markup, got:\n%s", out)
	}
	if strings.Contains(out, `id="graph"`) {
		t.Errorf("expected picker page to omit the graph canvas, got:\n%s", out)
	}
}

func TestRenderIndexEscapesBootstrapJSONForScriptContext(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{Mode: "single", Page: "viewer", ProjectKey: "</script><script>alert(1)</script>"}
	if err := RenderIndex(&buf, cfg); err != nil {
		t.Fatalf("RenderIndex() error = %v", err)
	}
	if strings.Contains(buf.String(), "</script><script>alert(1)</script>") {
		t.Error("expected bootstrap JSON to be HTML-escaped so it can't break out of the <script> tag")
	}
}

func TestIndexTemplateParsesOnce(t *testing.T) {
	t1 := IndexTemplate()
	t2 := IndexTemplate()
	if t1 != t2 {
		t.Error("expected IndexTemplate() to return the same cached *template.Template instance")
	}
}
