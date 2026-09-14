// Package mcpserver exposes the graph as MCP tools (query_graph, get_node,
// shortest_path, export_graph) for AI coding assistants, over stdio or
// HTTP transport — see design.md Decision 8: this package is a thin
// wrapper, all the intelligence lives in internal/context, internal/
// analytics, and internal/export.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	kcontext "github.com/josinaldojr/kgraph/internal/context"
	"github.com/josinaldojr/kgraph/internal/export"
	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
)

// Deps bundles what every tool handler needs: the loaded graph, the store
// its summaries live in, and the repo path export_graph needs for
// graph.json's metadata.
type Deps struct {
	Graph    *graph.Graph
	Store    *store.Store
	RepoPath string
}

// pathHopJSON is shortest_path's per-hop JSON shape, per mcp-server's
// "JSON array of hops, each with node_id, edge_type, and confidence"
// requirement.
type pathHopJSON struct {
	NodeID     string `json:"node_id"`
	NodeType   string `json:"node_type"`
	EdgeType   string `json:"edge_type,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// RegisterTools adds query_graph, get_node, shortest_path, and
// export_graph to s, backed by deps.
func RegisterTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("query_graph",
			mcp.WithDescription("Answer a natural-language question about the codebase with a relevant, token-budgeted subgraph — identical to `kgraph query`."),
			mcp.WithString("question", mcp.Required(), mcp.Description("The natural-language question to answer")),
		),
		queryGraphHandler(deps),
	)
	s.AddTool(
		mcp.NewTool("get_node",
			mcp.WithDescription("Get full details for a node: summary, relations, rationale, and analytics metadata — identical to `kgraph explain`."),
			mcp.WithString("id", mcp.Required(), mcp.Description("Node ID, file path, or name to resolve")),
		),
		getNodeHandler(deps),
	)
	s.AddTool(
		mcp.NewTool("shortest_path",
			mcp.WithDescription("Find the shortest weighted path between two nodes, returned as a JSON array of hops."),
			mcp.WithString("source", mcp.Required(), mcp.Description("Source node ID")),
			mcp.WithString("target", mcp.Required(), mcp.Description("Target node ID")),
		),
		shortestPathHandler(deps),
	)
	s.AddTool(
		mcp.NewTool("export_graph",
			mcp.WithDescription("Return the complete graph as JSON (nodes, edges, summaries, confidence, and analytics metadata) — equivalent to graph.json."),
		),
		exportGraphHandler(deps),
	)
}

func queryGraphHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		question, err := request.RequireString("question")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, err := kcontext.Query(deps.Graph, deps.Store, question, 0, 0)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	}
}

func getNodeHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, err := kcontext.Explain(deps.Graph, deps.Store, id, 1)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	}
}

func shortestPathHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		source, err := request.RequireString("source")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		target, err := request.RequireString("target")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hops, err := kcontext.Path(deps.Graph, source, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dtos := make([]pathHopJSON, len(hops))
		for i, h := range hops {
			dtos[i] = pathHopJSON{NodeID: h.Node.ID, NodeType: string(h.Node.Type), EdgeType: string(h.ViaEdge), Confidence: h.Confidence}
		}
		out, err := json.Marshal(dtos)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshaling path: %v", err)), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	}
}

func exportGraphHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := export.ToJSON(deps.Graph, deps.Store, deps.RepoPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	}
}
