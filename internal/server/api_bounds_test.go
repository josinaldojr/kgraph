package server

import (
	"net/http"
	"testing"
)

func TestIntParamClampsToMax(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		def  int
		max  int
		want int
	}{
		{"missing uses default", "", 2, 6, 2},
		{"non-numeric uses default", "abc", 2, 6, 2},
		{"zero uses default", "0", 2, 6, 2},
		{"negative uses default", "-5", 2, 6, 2},
		{"within range passes through", "4", 2, 6, 4},
		{"exactly at max passes through", "6", 2, 6, 6},
		{"over max clamps to max", "999999", 2, 6, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "http://example.com/?n="+tt.raw, nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if got := intParam(r, "n", tt.def, tt.max); got != tt.want {
				t.Errorf("intParam(%q, def=%d, max=%d) = %d, want %d", tt.raw, tt.def, tt.max, got, tt.want)
			}
		})
	}
}

func TestHandleGraphLocalOversizedHopsClamps(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto GraphDTO
	res := getJSON(t, srv.URL+"/api/graph/local?id=demo.Foo%28%29&hops=999999999", &dto)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for oversized hops, got %d", res.StatusCode)
	}
	if len(dto.Nodes) == 0 {
		t.Error("expected a non-empty local graph even with an oversized hops value")
	}
}

func TestHandleSearchOversizedTopKClamps(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var results []SearchResultDTO
	res := getJSON(t, srv.URL+"/api/search?q=invoice&topK=999999999", &results)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for oversized topK, got %d", res.StatusCode)
	}
	if len(results) > MaxTopK {
		t.Errorf("expected at most %d results, got %d", MaxTopK, len(results))
	}
}

func TestHandleQueryOversizedParamsClamp(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto QueryDTO
	res := getJSON(t, srv.URL+"/api/query?q=invoice&hops=999999999&max_tokens=999999999", &dto)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for oversized hops/max_tokens, got %d", res.StatusCode)
	}
	if dto.Result == "" {
		t.Error("expected a non-empty rendered result even with oversized params")
	}
}

func TestHandleExplainOversizedHopsClamps(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto ExplainDTO
	res := getJSON(t, srv.URL+"/api/explain?id=demo.Foo%28%29&hops=999999999", &dto)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for oversized hops, got %d", res.StatusCode)
	}
	if dto.Result == "" {
		t.Error("expected a non-empty rendered explain result even with an oversized hops value")
	}
}
