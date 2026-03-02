package guard

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/hir4ta/claude-pulse/internal/store"
)

// Engine evaluates tool inputs against guardrail rules.
type Engine struct {
	rules []compiledRule
}

type compiledRule struct {
	store.GuardrailRule
	re *regexp.Regexp
}

// CheckResult describes a guardrail match.
type CheckResult struct {
	Action  string // "block", "warn"
	Rule    store.GuardrailRule
	Matched string // excerpt of matching input
}

// NewEngine creates a guardrail engine from a set of rules.
// Rules with invalid regex patterns are silently skipped.
func NewEngine(rules []store.GuardrailRule) *Engine {
	var compiled []compiledRule
	for _, r := range rules {
		re, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, compiledRule{GuardrailRule: r, re: re})
	}
	return &Engine{rules: compiled}
}

// Check evaluates a tool invocation against all rules.
// Returns nil if no rule matches (silent pass).
func (e *Engine) Check(toolName string, toolInput json.RawMessage) *CheckResult {
	if len(e.rules) == 0 {
		return nil
	}

	texts := extractCheckTexts(toolName, toolInput)
	if len(texts) == 0 {
		return nil
	}

	for _, r := range e.rules {
		if !toolMatches(r.ToolName, toolName) {
			continue
		}

		for _, text := range texts {
			if loc := r.re.FindStringIndex(text); loc != nil {
				matched := text[loc[0]:loc[1]]
				if len(matched) > 100 {
					matched = matched[:100]
				}
				return &CheckResult{
					Action:  r.Action,
					Rule:    r.GuardrailRule,
					Matched: matched,
				}
			}
		}
	}

	return nil
}

// toolMatches checks if a rule's tool_name applies to the given tool.
func toolMatches(ruleToolName, actualTool string) bool {
	if ruleToolName == "*" {
		return true
	}
	// Support comma-separated tool names (e.g. "Write,Edit").
	for _, t := range strings.Split(ruleToolName, ",") {
		if strings.TrimSpace(t) == actualTool {
			return true
		}
	}
	return false
}

// extractCheckTexts pulls the relevant text fields from tool input for pattern matching.
func extractCheckTexts(toolName string, toolInput json.RawMessage) []string {
	if len(toolInput) == 0 {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(toolInput, &raw); err != nil {
		return []string{string(toolInput)}
	}

	var texts []string
	addStr := func(key string) {
		if v, ok := raw[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && s != "" {
				texts = append(texts, s)
			}
		}
	}

	switch toolName {
	case "Bash":
		addStr("command")
	case "Write":
		addStr("file_path")
		addStr("content")
	case "Edit":
		addStr("file_path")
		addStr("old_string")
		addStr("new_string")
	default:
		texts = append(texts, string(toolInput))
	}

	return texts
}
