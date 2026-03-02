package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hir4ta/claude-pulse/internal/guard"
	"github.com/hir4ta/claude-pulse/internal/store"
)

// debugWriter is set when PULSE_DEBUG is non-empty.
var debugWriter io.Writer

func init() {
	if os.Getenv("PULSE_DEBUG") == "" {
		return
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".claude-pulse")
	_ = os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(filepath.Join(dir, "debug.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	debugWriter = f
}

func debugf(format string, args ...any) {
	if debugWriter == nil {
		return
	}
	fmt.Fprintf(debugWriter, time.Now().Format("15:04:05.000")+" "+format+"\n", args...)
}

// hookEvent is the structure of a Claude Code hook stdin payload.
type hookEvent struct {
	SessionID   string          `json:"session_id"`
	ProjectPath string          `json:"cwd"`
	ToolName    string          `json:"tool_name"`
	ToolInput   json.RawMessage `json:"tool_input,omitempty"`
	ToolError   bool            `json:"tool_error"`
	Source      string          `json:"source"`
}

// runHook handles hook events from Claude Code.
func runHook(event string) error {
	debugf("hook event=%s", event)
	var ev hookEvent
	if err := json.NewDecoder(os.Stdin).Decode(&ev); err != nil {
		debugf("hook decode error: %v", err)
		return nil
	}
	debugf("hook session=%s tool=%s", ev.SessionID, ev.ToolName)

	// PreToolUse is the only hook that may produce output (blocking).
	if event == "PreToolUse" {
		return handlePreToolUse(&ev)
	}

	// All other hooks are silent data collection.
	st, err := store.OpenDefaultCached()
	if err != nil {
		debugf("hook store open failed: %v", err)
		return nil
	}

	switch event {
	case "SessionStart":
		if ev.SessionID != "" && ev.ProjectPath != "" {
			_ = st.EnsureSession(ev.SessionID, ev.ProjectPath)
			// Seed presets on first session (idempotent).
			_ = st.SeedPresets(guard.DefaultPresets())
		}
	case "PostToolUse":
		if ev.SessionID != "" && ev.ToolName != "" {
			_ = st.RecordToolUse(ev.SessionID, ev.ToolName, !ev.ToolError)
		}
	case "PostToolUseFailure":
		if ev.SessionID != "" && ev.ToolName != "" {
			_ = st.RecordToolUse(ev.SessionID, ev.ToolName, false)
		}
	case "SessionEnd":
		if ev.SessionID != "" {
			_ = st.EndSession(ev.SessionID)
		}
	}

	return nil
}

// handlePreToolUse checks tool input against guardrail rules.
// Outputs a blocking JSON response or nothing (silent pass).
func handlePreToolUse(ev *hookEvent) error {
	st, err := store.OpenDefaultCached()
	if err != nil {
		debugf("PreToolUse store open failed: %v", err)
		return nil // fail open — don't block if store is unavailable
	}

	rules, err := st.GetEnabledRules()
	if err != nil {
		debugf("PreToolUse get rules failed: %v", err)
		return nil
	}

	engine := guard.NewEngine(rules)
	result := engine.Check(ev.ToolName, ev.ToolInput)
	if result == nil {
		return nil // silent pass
	}

	debugf("PreToolUse matched rule=%s action=%s", result.Rule.Name, result.Action)

	// Log the guardrail action.
	if ev.SessionID != "" {
		_ = st.LogGuardrailAction(ev.SessionID, result.Rule.ID, ev.ToolName, result.Action, result.Matched)
	}

	// Output blocking response.
	var reason string
	switch result.Action {
	case "block":
		reason = fmt.Sprintf("⛔ %s", result.Rule.Message)
	case "warn":
		reason = fmt.Sprintf("⚠️ %s — ユーザーに確認してから再実行してください", result.Rule.Message)
	case "protect":
		reason = fmt.Sprintf("🔒 %s", result.Rule.Message)
	}

	resp := map[string]string{
		"decision": "block",
		"reason":   reason,
	}
	return json.NewEncoder(os.Stdout).Encode(resp)
}
