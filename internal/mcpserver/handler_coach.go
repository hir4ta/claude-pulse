package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hir4ta/claude-pulse/internal/coach"
	"github.com/hir4ta/claude-pulse/internal/embedder"
	"github.com/hir4ta/claude-pulse/internal/store"
)

func coachHandler(st *store.Store, emb *embedder.Embedder) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		wantTips := req.GetBool("tips", false)
		limit := req.GetInt("limit", 5)

		if wantTips {
			tips, err := coach.GenerateTips(st, "")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(toJSON(tips)), nil
		}

		if query != "" {
			results, err := coach.SearchBestPractices(ctx, st, emb, query, limit)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(results) == 0 {
				return mcp.NewToolResultText("No results found for: " + query), nil
			}
			return mcp.NewToolResultText(toJSON(results)), nil
		}

		return mcp.NewToolResultError("provide either 'query' for best practice search or 'tips': true for personalized tips"), nil
	}
}
