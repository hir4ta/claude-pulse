package mcpserver

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hir4ta/claude-pulse/internal/embedder"
	"github.com/hir4ta/claude-pulse/internal/store"
)

const serverInstructions = `pulse is your development health companion for Claude Code.

  stats  — View usage statistics and tool success rates
  guard  — Manage guardrail rules that protect against dangerous operations
  coach  — Get personalized tips and search Claude Code best practices
`

// New creates a new MCP server with all tools registered.
func New(st *store.Store, emb *embedder.Embedder) *server.MCPServer {
	s := server.NewMCPServer(
		"pulse",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(serverInstructions),
		server.WithLogging(),
	)

	s.AddTools(
		server.ServerTool{
			Tool: mcp.NewTool("stats",
				mcp.WithDescription("View usage statistics dashboard: tool success rates, session trends, and guardrail activity."),
				mcp.WithTitleAnnotation("Usage Statistics"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithString("period", mcp.Description("Time period: today, week, month, or all (default: week)")),
				mcp.WithString("project", mcp.Description("Project path filter (optional)")),
			),
			Handler: statsHandler(st),
		},

		server.ServerTool{
			Tool: mcp.NewTool("guard",
				mcp.WithDescription("Manage guardrail rules that protect against dangerous operations. List, add, remove, enable/disable rules, test patterns, or view action log."),
				mcp.WithTitleAnnotation("Guardrail Manager"),
				mcp.WithString("action", mcp.Required(), mcp.Description("Action: list, add, remove, enable, disable, test, log")),
				mcp.WithString("name", mcp.Description("Rule name (for add/remove/enable/disable)")),
				mcp.WithString("tool_name", mcp.Description("Tool name filter (for add, e.g. Bash, Write, Edit, *)")),
				mcp.WithString("pattern", mcp.Description("Regex pattern (for add/test)")),
				mcp.WithString("rule_action", mcp.Description("Rule action: block, warn, protect (for add)")),
				mcp.WithString("message", mcp.Description("Display message (for add)")),
				mcp.WithString("test_input", mcp.Description("Test input string (for test action)")),
				mcp.WithString("period", mcp.Description("Time period for log action: today, week, month, all (default: week)")),
			),
			Handler: guardHandler(st),
		},

		server.ServerTool{
			Tool: mcp.NewTool("coach",
				mcp.WithDescription("Get personalized development tips based on your usage data, or search Claude Code best practices."),
				mcp.WithTitleAnnotation("Development Coach"),
				mcp.WithReadOnlyHintAnnotation(true),
				mcp.WithString("query", mcp.Description("Search query for best practices (optional)")),
				mcp.WithBoolean("tips", mcp.Description("Generate personalized tips from usage data (default: false)")),
				mcp.WithNumber("limit", mcp.Description("Maximum search results (default: 5)")),
			),
			Handler: coachHandler(st, emb),
		},
	)

	return s
}
