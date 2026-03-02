package analytics

import (
	"path/filepath"
	"testing"
	"time"

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

func TestPeriodToTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		period   string
		wantDays float64 // approximate days ago from now
	}{
		{"today", "today", 1},
		{"week", "week", 7},
		{"month", "month", 30},
		{"unknown defaults to week", "bogus", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PeriodToTime(tt.period)
			diff := time.Since(got).Hours() / 24
			if diff < tt.wantDays-0.1 || diff > tt.wantDays+0.1 {
				t.Errorf("PeriodToTime(%q): %.1f days ago, want ~%.0f", tt.period, diff, tt.wantDays)
			}
		})
	}

	t.Run("all", func(t *testing.T) {
		t.Parallel()
		got := PeriodToTime("all")
		if got.Year() != 2000 {
			t.Errorf("PeriodToTime(all): year = %d, want 2000", got.Year())
		}
	})
}

func TestGenerateDashboard(t *testing.T) {
	t.Parallel()
	st := testStore(t)

	// Set up test data.
	_ = st.EnsureSession("s1", "/project")
	for range 5 {
		_ = st.RecordToolUse("s1", "Edit", true)
	}
	_ = st.RecordToolUse("s1", "Edit", false)
	_ = st.RecordToolUse("s1", "Bash", true)

	dashboard, err := GenerateDashboard(st, "all", "/project")
	if err != nil {
		t.Fatal(err)
	}

	// Sessions.
	if dashboard.Sessions.Total != 1 {
		t.Errorf("sessions.total = %d, want 1", dashboard.Sessions.Total)
	}

	// Tools: Edit should be first (6 uses > 1 for Bash).
	if len(dashboard.Tools.MostUsed) < 2 {
		t.Fatalf("most_used = %d tools, want >= 2", len(dashboard.Tools.MostUsed))
	}
	if dashboard.Tools.MostUsed[0].ToolName != "Edit" {
		t.Errorf("most_used[0] = %q, want Edit", dashboard.Tools.MostUsed[0].ToolName)
	}

	// HighestFailure: Edit has 1 failure / 6 total = 83% success < 90%.
	found := false
	for _, hf := range dashboard.Tools.HighestFailure {
		if hf.ToolName == "Edit" {
			found = true
		}
	}
	if !found {
		t.Error("Edit should appear in highest_failure (success_rate < 0.9)")
	}

	// Period.
	if dashboard.Period != "all" {
		t.Errorf("period = %q, want all", dashboard.Period)
	}
}

func TestGenerateDashboard_MostUsedCap(t *testing.T) {
	t.Parallel()
	st := testStore(t)

	_ = st.EnsureSession("s1", "/project")
	// Create 12 different tools.
	for i := range 12 {
		name := "Tool" + string(rune('A'+i))
		_ = st.RecordToolUse("s1", name, true)
	}

	dashboard, err := GenerateDashboard(st, "all", "/project")
	if err != nil {
		t.Fatal(err)
	}

	if len(dashboard.Tools.MostUsed) > 10 {
		t.Errorf("most_used = %d tools, want <= 10", len(dashboard.Tools.MostUsed))
	}
}

func TestGenerateDashboard_FormatJSON(t *testing.T) {
	t.Parallel()
	st := testStore(t)

	_ = st.EnsureSession("s1", "/project")
	dashboard, err := GenerateDashboard(st, "week", "")
	if err != nil {
		t.Fatal(err)
	}

	jsonStr, err := dashboard.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	if jsonStr == "" {
		t.Error("FormatJSON: got empty string")
	}
}
