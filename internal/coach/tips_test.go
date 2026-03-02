package coach

import (
	"path/filepath"
	"testing"

	"github.com/hir4ta/claude-pulse/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestCheckBashOveruse(t *testing.T) {
	t.Parallel()

	t.Run("overuse detected", func(t *testing.T) {
		t.Parallel()
		stats := []store.ToolStat{
			{ToolName: "Bash", TotalUses: 30, Successes: 30, SuccessRate: 1.0},
			{ToolName: "Edit", TotalUses: 10, Successes: 10, SuccessRate: 1.0},
		}
		tips := checkBashOveruse(stats)
		if len(tips) != 1 {
			t.Fatalf("checkBashOveruse: got %d tips, want 1", len(tips))
		}
		if tips[0].Category != "tool_usage" {
			t.Errorf("category = %q, want tool_usage", tips[0].Category)
		}
	})

	t.Run("no overuse", func(t *testing.T) {
		t.Parallel()
		stats := []store.ToolStat{
			{ToolName: "Bash", TotalUses: 10, Successes: 10, SuccessRate: 1.0},
			{ToolName: "Edit", TotalUses: 20, Successes: 20, SuccessRate: 1.0},
		}
		tips := checkBashOveruse(stats)
		if len(tips) != 0 {
			t.Errorf("checkBashOveruse: got %d tips, want 0", len(tips))
		}
	})

	t.Run("low total", func(t *testing.T) {
		t.Parallel()
		stats := []store.ToolStat{
			{ToolName: "Bash", TotalUses: 15, Successes: 15, SuccessRate: 1.0},
		}
		tips := checkBashOveruse(stats)
		if len(tips) != 0 {
			t.Errorf("checkBashOveruse with low total: got %d tips, want 0", len(tips))
		}
	})
}

func TestCheckHighFailureRate(t *testing.T) {
	t.Parallel()

	t.Run("high failure", func(t *testing.T) {
		t.Parallel()
		stats := []store.ToolStat{
			{ToolName: "Edit", TotalUses: 10, Successes: 5, Failures: 5, SuccessRate: 0.5},
		}
		tips := checkHighFailureRate(stats)
		if len(tips) != 1 {
			t.Fatalf("got %d tips, want 1", len(tips))
		}
		if tips[0].Severity != "warning" {
			t.Errorf("severity = %q, want warning", tips[0].Severity)
		}
	})

	t.Run("acceptable failure", func(t *testing.T) {
		t.Parallel()
		stats := []store.ToolStat{
			{ToolName: "Edit", TotalUses: 10, Successes: 8, Failures: 2, SuccessRate: 0.8},
		}
		tips := checkHighFailureRate(stats)
		if len(tips) != 0 {
			t.Errorf("got %d tips, want 0", len(tips))
		}
	})

	t.Run("too few uses", func(t *testing.T) {
		t.Parallel()
		stats := []store.ToolStat{
			{ToolName: "Edit", TotalUses: 3, Successes: 1, Failures: 2, SuccessRate: 0.33},
		}
		tips := checkHighFailureRate(stats)
		if len(tips) != 0 {
			t.Errorf("got %d tips, want 0 (below 5 use threshold)", len(tips))
		}
	})
}

func TestCheckLongSessions(t *testing.T) {
	t.Parallel()

	t.Run("long sessions", func(t *testing.T) {
		t.Parallel()
		summary := &store.SessionSummary{Total: 5, AvgTools: 250}
		tips := checkLongSessions(summary, nil)
		if len(tips) != 1 {
			t.Fatalf("got %d tips, want 1", len(tips))
		}
		if tips[0].Category != "session_health" {
			t.Errorf("category = %q, want session_health", tips[0].Category)
		}
	})

	t.Run("normal sessions", func(t *testing.T) {
		t.Parallel()
		summary := &store.SessionSummary{Total: 5, AvgTools: 50}
		tips := checkLongSessions(summary, nil)
		if len(tips) != 0 {
			t.Errorf("got %d tips, want 0", len(tips))
		}
	})

	t.Run("nil summary", func(t *testing.T) {
		t.Parallel()
		tips := checkLongSessions(nil, nil)
		if len(tips) != 0 {
			t.Errorf("got %d tips, want 0", len(tips))
		}
	})
}

func TestCheckRepeatedGuardrails(t *testing.T) {
	t.Parallel()

	t.Run("repeated triggers", func(t *testing.T) {
		t.Parallel()
		stats := []store.GuardrailStat{
			{RuleName: "rm-rf-root", Action: "block", Count: 5},
		}
		tips := checkRepeatedGuardrails(stats)
		if len(tips) != 1 {
			t.Fatalf("got %d tips, want 1", len(tips))
		}
		if tips[0].Severity != "warning" {
			t.Errorf("severity = %q, want warning", tips[0].Severity)
		}
	})

	t.Run("below threshold", func(t *testing.T) {
		t.Parallel()
		stats := []store.GuardrailStat{
			{RuleName: "rm-rf-root", Action: "block", Count: 2},
		}
		tips := checkRepeatedGuardrails(stats)
		if len(tips) != 0 {
			t.Errorf("got %d tips, want 0", len(tips))
		}
	})
}

func TestGenerateTips_NoIssues(t *testing.T) {
	t.Parallel()
	st := testStore(t)

	// Empty store: no usage data, should get default "all good" tip.
	tips, err := GenerateTips(st, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tips) != 1 {
		t.Fatalf("got %d tips, want 1 (default good tip)", len(tips))
	}
	if tips[0].Category != "general" {
		t.Errorf("category = %q, want general", tips[0].Category)
	}
	if tips[0].Severity != "info" {
		t.Errorf("severity = %q, want info", tips[0].Severity)
	}
}

func TestGenerateTips_BashOveruse(t *testing.T) {
	t.Parallel()
	st := testStore(t)

	_ = st.EnsureSession("s1", "/project")
	for range 25 {
		_ = st.RecordToolUse("s1", "Bash", true)
	}
	_ = st.RecordToolUse("s1", "Edit", true)

	tips, err := GenerateTips(st, "")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, tip := range tips {
		if tip.Category == "tool_usage" {
			found = true
		}
	}
	if !found {
		t.Error("expected tool_usage tip for Bash overuse")
	}
}
