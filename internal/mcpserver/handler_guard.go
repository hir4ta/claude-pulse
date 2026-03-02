package mcpserver

import (
	"context"
	"fmt"
	"regexp"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hir4ta/claude-pulse/internal/analytics"
	"github.com/hir4ta/claude-pulse/internal/store"
)

func guardHandler(st *store.Store) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		action := req.GetString("action", "")

		switch action {
		case "list":
			return guardList(st)
		case "add":
			return guardAdd(st, req)
		case "remove":
			return guardRemove(st, req)
		case "enable":
			return guardSetEnabled(st, req, true)
		case "disable":
			return guardSetEnabled(st, req, false)
		case "test":
			return guardTest(req)
		case "log":
			return guardLog(st, req)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("unknown action: %q (valid: list, add, remove, enable, disable, test, log)", action)), nil
		}
	}
}

func guardList(st *store.Store) (*mcp.CallToolResult, error) {
	rules, err := st.GetAllRules()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(rules) == 0 {
		return mcp.NewToolResultText("No guardrail rules configured."), nil
	}
	return mcp.NewToolResultText(toJSON(rules)), nil
}

func guardAdd(st *store.Store, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	toolName := req.GetString("tool_name", "")
	pattern := req.GetString("pattern", "")
	ruleAction := req.GetString("rule_action", "")
	message := req.GetString("message", "")

	if name == "" || toolName == "" || pattern == "" || ruleAction == "" || message == "" {
		return mcp.NewToolResultError("required fields: name, tool_name, pattern, rule_action, message"), nil
	}

	if _, err := regexp.Compile(pattern); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid regex pattern: %v", err)), nil
	}

	switch ruleAction {
	case "block", "warn", "protect":
	default:
		return mcp.NewToolResultError(fmt.Sprintf("invalid rule_action: %q (valid: block, warn, protect)", ruleAction)), nil
	}

	id, err := st.AddGuardrailRule(&store.GuardrailRule{
		Name:     name,
		ToolName: toolName,
		Pattern:  pattern,
		Action:   ruleAction,
		Severity: "medium",
		Message:  message,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(toJSON(map[string]any{
		"status": "added",
		"id":     id,
		"name":   name,
	})), nil
}

func guardRemove(st *store.Store, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := st.RemoveGuardrailRule(name); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(toJSON(map[string]string{
		"status": "removed",
		"name":   name,
	})), nil
}

func guardSetEnabled(st *store.Store, req mcp.CallToolRequest, enabled bool) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := st.SetGuardrailRuleEnabled(name, enabled); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	status := "enabled"
	if !enabled {
		status = "disabled"
	}
	return mcp.NewToolResultText(toJSON(map[string]string{
		"status": status,
		"name":   name,
	})), nil
}

func guardTest(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pattern := req.GetString("pattern", "")
	testInput := req.GetString("test_input", "")
	toolName := req.GetString("tool_name", "Bash")

	if pattern == "" || testInput == "" {
		return mcp.NewToolResultError("pattern and test_input are required"), nil
	}

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid regex: %v", err)), nil
	}

	match := re.FindString(testInput)
	result := map[string]any{
		"pattern":    pattern,
		"test_input": testInput,
		"tool_name":  toolName,
		"matched":    match != "",
	}
	if match != "" {
		result["match"] = match
	}

	return mcp.NewToolResultText(toJSON(result)), nil
}

func guardLog(st *store.Store, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	period := req.GetString("period", "week")
	since := analytics.PeriodToTime(period)

	entries, err := st.GetGuardrailLog(since, 50)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(entries) == 0 {
		return mcp.NewToolResultText("No guardrail actions logged in this period."), nil
	}

	return mcp.NewToolResultText(toJSON(entries)), nil
}
