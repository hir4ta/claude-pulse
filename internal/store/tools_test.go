package store

import (
	"testing"
	"time"
)

func TestEnsureSessionAndRecordToolUse(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	if err := st.EnsureSession("s1", "/tmp/project"); err != nil {
		t.Fatal(err)
	}

	// Record some tool uses.
	if err := st.RecordToolUse("s1", "Edit", true); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordToolUse("s1", "Edit", true); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordToolUse("s1", "Edit", false); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordToolUse("s1", "Bash", true); err != nil {
		t.Fatal(err)
	}

	// Check tool stats.
	stats, err := st.GetToolStats("/tmp/project", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("GetToolStats: got %d tools, want 2", len(stats))
	}

	// Edit should be first (3 total uses > 1 for Bash).
	if stats[0].ToolName != "Edit" {
		t.Errorf("most used tool = %q, want Edit", stats[0].ToolName)
	}
	if stats[0].Successes != 2 || stats[0].Failures != 1 {
		t.Errorf("Edit: success=%d fail=%d, want 2/1", stats[0].Successes, stats[0].Failures)
	}
}

func TestEndSession(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	_ = st.EnsureSession("s1", "/tmp/project")
	if err := st.EndSession("s1"); err != nil {
		t.Fatal(err)
	}

	var endedAt *string
	err := st.DB().QueryRow("SELECT ended_at FROM sessions WHERE id = 's1'").Scan(&endedAt)
	if err != nil {
		t.Fatal(err)
	}
	if endedAt == nil {
		t.Error("ended_at should not be nil after EndSession")
	}
}

func TestSessionSummary(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	_ = st.EnsureSession("s1", "/tmp/project")
	_ = st.RecordToolUse("s1", "Edit", true)
	_ = st.RecordToolUse("s1", "Bash", true)

	summary, err := st.GetSessionSummary("/tmp/project", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 1 {
		t.Errorf("total sessions = %d, want 1", summary.Total)
	}
}
