package store

import (
	"fmt"
	"time"
)

// EnsureSession creates a session row if it doesn't already exist.
func (s *Store) EnsureSession(sessionID, projectPath string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO sessions (id, project_path)
		VALUES (?, ?)`,
		sessionID, projectPath,
	)
	if err != nil {
		return fmt.Errorf("store: ensure session: %w", err)
	}
	return nil
}

// EndSession sets the ended_at timestamp for a session.
func (s *Store) EndSession(sessionID string) error {
	_, err := s.db.Exec(`
		UPDATE sessions SET ended_at = datetime('now')
		WHERE id = ? AND ended_at IS NULL`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("store: end session: %w", err)
	}
	return nil
}

// RecordToolUse increments success or failure count for a tool in a session.
// Also increments the session-level counters.
func (s *Store) RecordToolUse(sessionID, toolName string, success bool) error {
	now := time.Now().UTC().Format(time.RFC3339)

	successInc := 0
	failureInc := 0
	if success {
		successInc = 1
	} else {
		failureInc = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO tool_stats (session_id, tool_name, success, failure, last_used)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(session_id, tool_name) DO UPDATE SET
			success = success + excluded.success,
			failure = failure + excluded.failure,
			last_used = excluded.last_used`,
		sessionID, toolName, successInc, failureInc, now,
	)
	if err != nil {
		return fmt.Errorf("store: record tool use: %w", err)
	}

	// Update session-level counters.
	_, err = s.db.Exec(`
		UPDATE sessions SET
			tool_use_count = tool_use_count + 1,
			tool_fail_count = tool_fail_count + ?
		WHERE id = ?`,
		failureInc, sessionID,
	)
	if err != nil {
		return fmt.Errorf("store: update session counters: %w", err)
	}
	return nil
}

// ToolStat represents aggregated tool usage statistics.
type ToolStat struct {
	ToolName    string  `json:"name"`
	TotalUses   int     `json:"total_uses"`
	Successes   int     `json:"successes"`
	Failures    int     `json:"failures"`
	SuccessRate float64 `json:"success_rate"`
}

// GetToolStats returns aggregated tool statistics for a project within a time range.
// If projectPath is empty, returns stats across all projects.
// sinceSQL is a datetime string (e.g. datetime('now', '-7 days')).
func (s *Store) GetToolStats(projectPath string, sinceSQL string) ([]ToolStat, error) {
	var query string
	var args []any

	if projectPath != "" {
		query = `
			SELECT ts.tool_name,
			       SUM(ts.success) as total_success,
			       SUM(ts.failure) as total_failure
			FROM tool_stats ts
			JOIN sessions s ON s.id = ts.session_id
			WHERE s.project_path = ? AND s.started_at >= ` + sinceSQL + `
			GROUP BY ts.tool_name
			ORDER BY (SUM(ts.success) + SUM(ts.failure)) DESC`
		args = []any{projectPath}
	} else {
		query = `
			SELECT ts.tool_name,
			       SUM(ts.success) as total_success,
			       SUM(ts.failure) as total_failure
			FROM tool_stats ts
			JOIN sessions s ON s.id = ts.session_id
			WHERE s.started_at >= ` + sinceSQL + `
			GROUP BY ts.tool_name
			ORDER BY (SUM(ts.success) + SUM(ts.failure)) DESC`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: get tool stats: %w", err)
	}
	defer rows.Close()

	var stats []ToolStat
	for rows.Next() {
		var ts ToolStat
		if err := rows.Scan(&ts.ToolName, &ts.Successes, &ts.Failures); err != nil {
			continue
		}
		ts.TotalUses = ts.Successes + ts.Failures
		if ts.TotalUses > 0 {
			ts.SuccessRate = float64(ts.Successes) / float64(ts.TotalUses)
		}
		stats = append(stats, ts)
	}
	return stats, nil
}

// SessionSummary holds basic session statistics.
type SessionSummary struct {
	Total    int `json:"total"`
	AvgTools int `json:"avg_tools_per_session"`
}

// GetSessionSummary returns session count and average tool usage for a period.
func (s *Store) GetSessionSummary(projectPath string, sinceSQL string) (*SessionSummary, error) {
	var query string
	var args []any

	if projectPath != "" {
		query = `
			SELECT COUNT(*), COALESCE(AVG(tool_use_count), 0)
			FROM sessions
			WHERE project_path = ? AND started_at >= ` + sinceSQL
		args = []any{projectPath}
	} else {
		query = `
			SELECT COUNT(*), COALESCE(AVG(tool_use_count), 0)
			FROM sessions
			WHERE started_at >= ` + sinceSQL
	}

	var summary SessionSummary
	var avgFloat float64
	err := s.db.QueryRow(query, args...).Scan(&summary.Total, &avgFloat)
	if err != nil {
		return nil, fmt.Errorf("store: get session summary: %w", err)
	}
	summary.AvgTools = int(avgFloat)
	return &summary, nil
}
