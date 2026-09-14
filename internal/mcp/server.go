package mcpserver

import (
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// NewServer builds the MCP server (tool capabilities enabled, panics in a
// handler recovered rather than crashing the process) with every tool
// registered against deps.
func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer("kgraph", "1.0.0", server.WithToolCapabilities(false), server.WithRecovery())
	RegisterTools(s, deps)
	return s
}

// ServeStdio runs s over stdio until the client disconnects — per
// mcp-server's "MCP server starts via stdio" requirement.
func ServeStdio(s *server.MCPServer) error {
	return server.ServeStdio(s)
}

// ServeHTTP starts s over streamable-HTTP on addr (e.g. ":8080"), blocking
// until the listener stops — per mcp-server's "MCP server starts via
// HTTP" requirement. When apiKey is non-empty, every request must carry
// `Authorization: Bearer <apiKey>` or it's rejected with 401, per
// mcp-server's HTTP authentication requirement; an empty apiKey accepts
// all requests unauthenticated.
func ServeHTTP(s *server.MCPServer, addr, apiKey string) error {
	httpServer := server.NewStreamableHTTPServer(s)
	var handler http.Handler = httpServer
	if apiKey != "" {
		handler = requireBearer(apiKey, handler)
	}
	return http.ListenAndServe(addr, handler)
}

// requireBearer rejects any request whose Authorization header isn't
// exactly "Bearer <apiKey>" with 401, before it reaches next.
func requireBearer(apiKey string, next http.Handler) http.Handler {
	want := "Bearer " + apiKey
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != want {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintln(w, `{"error":"unauthorized"}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}
