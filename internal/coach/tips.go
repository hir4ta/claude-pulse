package coach

import (
	"fmt"
	"time"

	"github.com/hir4ta/claude-pulse/internal/store"
)

// Tip is a personalized development advice.
type Tip struct {
	Category string `json:"category"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "info", "warning", "suggestion"
}

// GenerateTips analyzes usage data and returns personalized tips.
// This is entirely rule-based (no LLM needed).
func GenerateTips(st *store.Store, projectPath string) ([]Tip, error) {
	since := time.Now().UTC().Add(-7 * 24 * time.Hour)

	tools, err := st.GetToolStats(projectPath, since)
	if err != nil {
		return nil, fmt.Errorf("coach: get tool stats: %w", err)
	}

	sessions, err := st.GetSessionSummary(projectPath, since)
	if err != nil {
		return nil, fmt.Errorf("coach: get session summary: %w", err)
	}

	guardrails, err := st.GetGuardrailStats(since)
	if err != nil {
		guardrails = nil
	}

	var tips []Tip

	// Rule 1: Bash overuse pattern.
	tips = append(tips, checkBashOveruse(tools)...)

	// Rule 2: High failure rate tools.
	tips = append(tips, checkHighFailureRate(tools)...)

	// Rule 3: Long sessions.
	tips = append(tips, checkLongSessions(sessions, tools)...)

	// Rule 4: Repeated guardrail triggers.
	tips = append(tips, checkRepeatedGuardrails(guardrails)...)

	if len(tips) == 0 {
		tips = append(tips, Tip{
			Category: "general",
			Message:  "直近1週間の使用状況は良好です。特に改善すべき点は見つかりませんでした。",
			Severity: "info",
		})
	}

	return tips, nil
}

func checkBashOveruse(tools []store.ToolStat) []Tip {
	totalUses := 0
	bashUses := 0
	for _, t := range tools {
		totalUses += t.TotalUses
		if t.ToolName == "Bash" {
			bashUses = t.TotalUses
		}
	}

	if totalUses > 20 && bashUses > 0 {
		ratio := float64(bashUses) / float64(totalUses)
		if ratio > 0.5 {
			return []Tip{{
				Category: "tool_usage",
				Message:  fmt.Sprintf("Bash の使用率が %.0f%% と高めです。Read/Edit/Grep 等の専用ツールを活用すると安全性と可読性が向上します。", ratio*100),
				Severity: "suggestion",
			}}
		}
	}
	return nil
}

func checkHighFailureRate(tools []store.ToolStat) []Tip {
	var tips []Tip
	for _, t := range tools {
		if t.TotalUses >= 5 && t.SuccessRate < 0.7 {
			tips = append(tips, Tip{
				Category: "tool_failure",
				Message:  fmt.Sprintf("%s の失敗率が %.0f%% です。入力パラメータやファイルパスを確認してください。", t.ToolName, (1-t.SuccessRate)*100),
				Severity: "warning",
			})
		}
	}
	return tips
}

func checkLongSessions(sessions *store.SessionSummary, tools []store.ToolStat) []Tip {
	if sessions == nil {
		return nil
	}
	// If average tools per session is very high, suggest splitting.
	if sessions.AvgTools > 200 {
		return []Tip{{
			Category: "session_health",
			Message:  fmt.Sprintf("セッションあたりの平均ツール使用回数が %d 回と多めです。タスクを分割するとコンテキストの compact を避けられます。", sessions.AvgTools),
			Severity: "suggestion",
		}}
	}
	return nil
}

func checkRepeatedGuardrails(stats []store.GuardrailStat) []Tip {
	var tips []Tip
	for _, gs := range stats {
		if gs.Count >= 3 {
			tips = append(tips, Tip{
				Category: "guardrail",
				Message:  fmt.Sprintf("ルール「%s」が %d 回発動しています。ワークフローの見直しを検討してください。", gs.RuleName, gs.Count),
				Severity: "warning",
			})
		}
	}
	return tips
}
