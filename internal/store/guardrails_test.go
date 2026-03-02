package store

import (
	"testing"
	"time"
)

func TestSeedPresetsAndGetEnabled(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	presets := []GuardrailRule{
		{Name: "test-block", ToolName: "Bash", Pattern: `rm -rf`, Action: "block", Severity: "critical", Message: "test block"},
		{Name: "test-warn", ToolName: "Bash", Pattern: `--force`, Action: "warn", Severity: "high", Message: "test warn"},
	}
	if err := st.SeedPresets(presets); err != nil {
		t.Fatal(err)
	}

	rules, err := st.GetEnabledRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("GetEnabledRules: got %d, want 2", len(rules))
	}

	// Block should come first (ordering by action priority).
	if rules[0].Action != "block" {
		t.Errorf("first rule action = %q, want block", rules[0].Action)
	}
}

func TestAddAndRemoveCustomRule(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	id, err := st.AddGuardrailRule(&GuardrailRule{
		Name:     "custom-rule",
		ToolName: "Bash",
		Pattern:  `danger`,
		Action:   "block",
		Severity: "high",
		Message:  "custom block",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Error("AddGuardrailRule returned id=0")
	}

	if err := st.RemoveGuardrailRule("custom-rule"); err != nil {
		t.Fatal(err)
	}

	// Verify it's gone.
	rules, _ := st.GetAllRules()
	for _, r := range rules {
		if r.Name == "custom-rule" {
			t.Error("custom-rule should have been removed")
		}
	}
}

func TestDisableEnableRule(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	_ = st.SeedPresets([]GuardrailRule{
		{Name: "toggle-me", ToolName: "Bash", Pattern: `test`, Action: "warn", Severity: "low", Message: "toggle"},
	})

	if err := st.SetGuardrailRuleEnabled("toggle-me", false); err != nil {
		t.Fatal(err)
	}

	enabled, _ := st.GetEnabledRules()
	for _, r := range enabled {
		if r.Name == "toggle-me" {
			t.Error("disabled rule should not appear in GetEnabledRules")
		}
	}

	if err := st.SetGuardrailRuleEnabled("toggle-me", true); err != nil {
		t.Fatal(err)
	}
}

func TestLogGuardrailAction(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	_ = st.EnsureSession("s1", "/tmp/project")
	_ = st.SeedPresets([]GuardrailRule{
		{Name: "log-test", ToolName: "Bash", Pattern: `rm`, Action: "block", Severity: "critical", Message: "blocked"},
	})

	rules, _ := st.GetEnabledRules()
	if len(rules) == 0 {
		t.Fatal("no rules")
	}

	if err := st.LogGuardrailAction("s1", rules[0].ID, "Bash", "block", "rm -rf /"); err != nil {
		t.Fatal(err)
	}

	entries, err := st.GetGuardrailLog(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("GetGuardrailLog: got %d, want 1", len(entries))
	}
	if entries[0].Action != "block" {
		t.Errorf("entry action = %q, want block", entries[0].Action)
	}
}
