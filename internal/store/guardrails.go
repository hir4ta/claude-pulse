package store

import (
	"fmt"
	"time"
)

// GuardrailRule represents a row in the guardrail_rules table.
type GuardrailRule struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ToolName string `json:"tool_name"`
	Pattern  string `json:"pattern"`
	Action   string `json:"action"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Enabled  bool   `json:"enabled"`
	IsPreset bool   `json:"is_preset"`
}

// GetEnabledRules returns all enabled guardrail rules.
func (s *Store) GetEnabledRules() ([]GuardrailRule, error) {
	rows, err := s.db.Query(`
		SELECT id, name, tool_name, pattern, action, severity, message, enabled, is_preset
		FROM guardrail_rules
		WHERE enabled = 1
		ORDER BY
			CASE action WHEN 'block' THEN 0 WHEN 'protect' THEN 1 WHEN 'warn' THEN 2 END,
			id`)
	if err != nil {
		return nil, fmt.Errorf("store: get enabled rules: %w", err)
	}
	defer rows.Close()

	var rules []GuardrailRule
	for rows.Next() {
		var r GuardrailRule
		if err := rows.Scan(&r.ID, &r.Name, &r.ToolName, &r.Pattern, &r.Action,
			&r.Severity, &r.Message, &r.Enabled, &r.IsPreset); err != nil {
			return nil, fmt.Errorf("store: scan enabled rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate enabled rules: %w", err)
	}
	return rules, nil
}

// GetAllRules returns all guardrail rules regardless of enabled status.
func (s *Store) GetAllRules() ([]GuardrailRule, error) {
	rows, err := s.db.Query(`
		SELECT id, name, tool_name, pattern, action, severity, message, enabled, is_preset
		FROM guardrail_rules
		ORDER BY is_preset DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("store: get all rules: %w", err)
	}
	defer rows.Close()

	var rules []GuardrailRule
	for rows.Next() {
		var r GuardrailRule
		if err := rows.Scan(&r.ID, &r.Name, &r.ToolName, &r.Pattern, &r.Action,
			&r.Severity, &r.Message, &r.Enabled, &r.IsPreset); err != nil {
			return nil, fmt.Errorf("store: scan rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate all rules: %w", err)
	}
	return rules, nil
}

// AddGuardrailRule inserts a new custom guardrail rule.
func (s *Store) AddGuardrailRule(r *GuardrailRule) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO guardrail_rules (name, tool_name, pattern, action, severity, message, enabled, is_preset)
		VALUES (?, ?, ?, ?, ?, ?, 1, 0)`,
		r.Name, r.ToolName, r.Pattern, r.Action, r.Severity, r.Message,
	)
	if err != nil {
		return 0, fmt.Errorf("store: add guardrail rule: %w", err)
	}
	return res.LastInsertId()
}

// RemoveGuardrailRule deletes a custom (non-preset) rule by name.
func (s *Store) RemoveGuardrailRule(name string) error {
	res, err := s.db.Exec(`DELETE FROM guardrail_rules WHERE name = ? AND is_preset = 0`, name)
	if err != nil {
		return fmt.Errorf("store: remove guardrail rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule %q not found or is a preset (cannot delete presets)", name)
	}
	return nil
}

// SetGuardrailRuleEnabled enables or disables a rule by name.
func (s *Store) SetGuardrailRuleEnabled(name string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	res, err := s.db.Exec(`UPDATE guardrail_rules SET enabled = ? WHERE name = ?`, val, name)
	if err != nil {
		return fmt.Errorf("store: set guardrail enabled: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule %q not found", name)
	}
	return nil
}

// SeedPresets inserts preset rules if they don't already exist.
func (s *Store) SeedPresets(presets []GuardrailRule) error {
	for _, p := range presets {
		_, err := s.db.Exec(`
			INSERT OR IGNORE INTO guardrail_rules (name, tool_name, pattern, action, severity, message, enabled, is_preset)
			VALUES (?, ?, ?, ?, ?, ?, 1, 1)`,
			p.Name, p.ToolName, p.Pattern, p.Action, p.Severity, p.Message,
		)
		if err != nil {
			return fmt.Errorf("store: seed preset %s: %w", p.Name, err)
		}
	}
	return nil
}

// LogGuardrailAction records a guardrail action in the log.
func (s *Store) LogGuardrailAction(sessionID string, ruleID int64, toolName, action, matched string) error {
	_, err := s.db.Exec(`
		INSERT INTO guardrail_log (session_id, rule_id, tool_name, action, matched, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, ruleID, toolName, action, matched, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("store: log guardrail action: %w", err)
	}
	return nil
}

// GuardrailLogEntry represents a logged guardrail action.
type GuardrailLogEntry struct {
	ID        int64  `json:"id"`
	RuleName  string `json:"rule_name"`
	ToolName  string `json:"tool_name"`
	Action    string `json:"action"`
	Matched   string `json:"matched"`
	Timestamp string `json:"timestamp"`
}

// GetGuardrailLog returns recent guardrail actions.
func (s *Store) GetGuardrailLog(since time.Time, limit int) ([]GuardrailLogEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT gl.id, gr.name, gl.tool_name, gl.action, gl.matched, gl.timestamp
		FROM guardrail_log gl
		JOIN guardrail_rules gr ON gr.id = gl.rule_id
		WHERE gl.timestamp >= ?
		ORDER BY gl.timestamp DESC
		LIMIT ?`, formatSince(since), limit)
	if err != nil {
		return nil, fmt.Errorf("store: get guardrail log: %w", err)
	}
	defer rows.Close()

	var entries []GuardrailLogEntry
	for rows.Next() {
		var e GuardrailLogEntry
		if err := rows.Scan(&e.ID, &e.RuleName, &e.ToolName, &e.Action, &e.Matched, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("store: scan guardrail log: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate guardrail log: %w", err)
	}
	return entries, nil
}

// GuardrailStat represents aggregated guardrail action counts.
type GuardrailStat struct {
	RuleName string `json:"name"`
	Action   string `json:"action"`
	Count    int    `json:"count"`
}

// GetGuardrailStats returns aggregated guardrail action counts for a period.
func (s *Store) GetGuardrailStats(since time.Time) ([]GuardrailStat, error) {
	rows, err := s.db.Query(`
		SELECT gr.name, gl.action, COUNT(*)
		FROM guardrail_log gl
		JOIN guardrail_rules gr ON gr.id = gl.rule_id
		WHERE gl.timestamp >= ?
		GROUP BY gr.name, gl.action
		ORDER BY COUNT(*) DESC`, formatSince(since))
	if err != nil {
		return nil, fmt.Errorf("store: get guardrail stats: %w", err)
	}
	defer rows.Close()

	var stats []GuardrailStat
	for rows.Next() {
		var gs GuardrailStat
		if err := rows.Scan(&gs.RuleName, &gs.Action, &gs.Count); err != nil {
			return nil, fmt.Errorf("store: scan guardrail stats: %w", err)
		}
		stats = append(stats, gs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate guardrail stats: %w", err)
	}
	return stats, nil
}
