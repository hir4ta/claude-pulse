package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hir4ta/claude-pulse/internal/analytics"
	"github.com/hir4ta/claude-pulse/internal/store"
)

func statsHandler(st *store.Store) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		period := req.GetString("period", "week")
		project := req.GetString("project", "")

		dashboard, err := analytics.GenerateDashboard(st, period, project)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonStr, err := dashboard.FormatJSON()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(jsonStr), nil
	}
}

// toJSON marshals a value as indented JSON.
func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}
