package analytics

import (
	"encoding/json"
	"fmt"

	"github.com/hir4ta/claude-pulse/internal/store"
)

// Dashboard is the top-level stats response.
type Dashboard struct {
	Period     string          `json:"period"`
	Sessions   *store.SessionSummary `json:"sessions"`
	Tools      ToolsSection    `json:"tools"`
	Guardrails GuardrailSection `json:"guardrails"`
}

// ToolsSection groups tool statistics.
type ToolsSection struct {
	MostUsed       []store.ToolStat `json:"most_used"`
	HighestFailure []store.ToolStat `json:"highest_failure,omitempty"`
}

// GuardrailSection groups guardrail statistics.
type GuardrailSection struct {
	TotalBlocked int                  `json:"total_blocked"`
	TotalWarned  int                  `json:"total_warned"`
	ByRule       []store.GuardrailStat `json:"by_rule,omitempty"`
}

// PeriodToSQL converts a period name to a SQL datetime expression.
func PeriodToSQL(period string) string {
	switch period {
	case "today":
		return "datetime('now', '-1 day')"
	case "week":
		return "datetime('now', '-7 days')"
	case "month":
		return "datetime('now', '-30 days')"
	case "all":
		return "datetime('2000-01-01')"
	default:
		return "datetime('now', '-7 days')"
	}
}

// GenerateDashboard builds the stats dashboard for a given period and project.
func GenerateDashboard(st *store.Store, period, projectPath string) (*Dashboard, error) {
	sinceSQL := PeriodToSQL(period)

	sessions, err := st.GetSessionSummary(projectPath, sinceSQL)
	if err != nil {
		return nil, fmt.Errorf("analytics: session summary: %w", err)
	}

	tools, err := st.GetToolStats(projectPath, sinceSQL)
	if err != nil {
		return nil, fmt.Errorf("analytics: tool stats: %w", err)
	}

	// Split into most_used and highest_failure.
	var highFailure []store.ToolStat
	for _, t := range tools {
		if t.Failures > 0 && t.SuccessRate < 0.9 {
			highFailure = append(highFailure, t)
		}
	}

	// Limit to top 10 most used.
	mostUsed := tools
	if len(mostUsed) > 10 {
		mostUsed = mostUsed[:10]
	}

	gStats, err := st.GetGuardrailStats(sinceSQL)
	if err != nil {
		gStats = nil // non-fatal
	}

	totalBlocked := 0
	totalWarned := 0
	for _, gs := range gStats {
		switch gs.Action {
		case "block", "protect":
			totalBlocked += gs.Count
		case "warn":
			totalWarned += gs.Count
		}
	}

	return &Dashboard{
		Period:   period,
		Sessions: sessions,
		Tools: ToolsSection{
			MostUsed:       mostUsed,
			HighestFailure: highFailure,
		},
		Guardrails: GuardrailSection{
			TotalBlocked: totalBlocked,
			TotalWarned:  totalWarned,
			ByRule:       gStats,
		},
	}, nil
}

// FormatJSON returns the dashboard as indented JSON.
func (d *Dashboard) FormatJSON() (string, error) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
