package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoverMiddlewareIsolatesPanicToOneRequest(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux := http.NewServeMux()
	mux.Handle("/panic", panicking)
	mux.Handle("/ok", ok)
	srv := httptest.NewServer(recoverMiddleware(mux))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/panic")
	if err != nil {
		t.Fatalf("GET /panic: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 from a panicking handler, got %d", res.StatusCode)
	}

	res2, err := http.Get(srv.URL + "/ok")
	if err != nil {
		t.Fatalf("GET /ok after panic: %v", err)
	}
	res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		t.Errorf("expected the server to keep serving after a recovered panic, got %d", res2.StatusCode)
	}
}
