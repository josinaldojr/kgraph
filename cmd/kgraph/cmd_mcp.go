package main

import (
	"fmt"

	"github.com/spf13/cobra"

	mcpserver "kgraph/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	var transport string
	var port int
	var apiKey string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the graph as MCP tools (query_graph, get_node, shortest_path, export_graph) for AI assistants",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, dbPath, err := resolvePaths()
			if err != nil {
				return fail(cmd, err)
			}
			g, s, closeFn, err := loadGraphAndStore(dbPath)
			if err != nil {
				return fail(cmd, err)
			}
			defer closeFn()

			srv := mcpserver.NewServer(mcpserver.Deps{Graph: g, Store: s, RepoPath: repoPath})

			switch transport {
			case "stdio":
				return mcpserver.ServeStdio(srv)
			case "http":
				addr := fmt.Sprintf(":%d", port)
				fmt.Fprintf(cmd.OutOrStdout(), "kgraph mcp: listening on http://localhost%s\n", addr)
				return mcpserver.ServeHTTP(srv, addr, apiKey)
			default:
				return fail(cmd, fmt.Errorf("mcp: unknown --transport %q, want \"stdio\" or \"http\"", transport))
			}
		},
	}
	cmd.Flags().StringVar(&transport, "transport", "stdio", `MCP transport: "stdio" or "http"`)
	cmd.Flags().IntVar(&port, "port", 8080, "port to listen on (--transport http only)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "require this API key on HTTP requests (--transport http only; no auth if empty)")
	return cmd
}
