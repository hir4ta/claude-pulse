package guard

import "github.com/hir4ta/claude-pulse/internal/store"

// DefaultPresets returns the built-in guardrail rules.
func DefaultPresets() []store.GuardrailRule {
	return []store.GuardrailRule{
		// ============================================================
		// Block — immediately prevent dangerous commands
		// ============================================================
		{
			Name:     "rm-rf-root",
			ToolName: "Bash",
			Pattern:  `rm\s+-rf\s+/[^.\s]`,
			Action:   "block",
			Severity: "critical",
			Message:  "危険: ルートディレクトリの再帰削除を検出しました",
		},
		{
			Name:     "fork-bomb",
			ToolName: "Bash",
			Pattern:  `:\(\)\s*\{\s*:\|:&\s*\}\s*;`,
			Action:   "block",
			Severity: "critical",
			Message:  "危険: fork bomb を検出しました",
		},
		{
			Name:     "mkfs",
			ToolName: "Bash",
			Pattern:  `mkfs\.`,
			Action:   "block",
			Severity: "critical",
			Message:  "危険: ファイルシステムフォーマットを検出しました",
		},
		{
			Name:     "dd-device",
			ToolName: "Bash",
			Pattern:  `dd\s+if=.*/dev/`,
			Action:   "block",
			Severity: "critical",
			Message:  "危険: デバイスへの直接書き込みを検出しました",
		},
		{
			Name:     "dev-sda-write",
			ToolName: "Bash",
			Pattern:  `>\s*/dev/sd[a-z]`,
			Action:   "block",
			Severity: "critical",
			Message:  "危険: ブロックデバイスへの書き込みを検出しました",
		},
		{
			Name:     "chmod-777-root",
			ToolName: "Bash",
			Pattern:  `chmod\s+-R\s+777\s+/`,
			Action:   "block",
			Severity: "critical",
			Message:  "危険: ルートの権限を全開放する操作を検出しました",
		},

		// ============================================================
		// Warn — prompt user confirmation before proceeding
		// ============================================================
		{
			Name:     "force-push",
			ToolName: "Bash",
			Pattern:  `git\s+push\s+.*--force`,
			Action:   "warn",
			Severity: "high",
			Message:  "git force push を検出しました。意図した操作か確認してください",
		},
		{
			Name:     "hard-reset",
			ToolName: "Bash",
			Pattern:  `git\s+reset\s+--hard`,
			Action:   "warn",
			Severity: "high",
			Message:  "git hard reset を検出しました。未コミットの変更が失われます",
		},
		{
			Name:     "drop-table",
			ToolName: "Bash",
			Pattern:  `DROP\s+TABLE`,
			Action:   "warn",
			Severity: "high",
			Message:  "DROP TABLE を検出しました。データベーステーブルが削除されます",
		},
		{
			Name:     "truncate-table",
			ToolName: "Bash",
			Pattern:  `TRUNCATE\s+`,
			Action:   "warn",
			Severity: "high",
			Message:  "TRUNCATE を検出しました。テーブルの全データが削除されます",
		},
		{
			Name:     "no-verify",
			ToolName: "Bash",
			Pattern:  `--no-verify`,
			Action:   "warn",
			Severity: "medium",
			Message:  "--no-verify を検出しました。pre-commit hook がスキップされます",
		},
		{
			Name:     "branch-delete-force",
			ToolName: "Bash",
			Pattern:  `git\s+branch\s+-D\s+`,
			Action:   "warn",
			Severity: "medium",
			Message:  "git branch -D を検出しました。ブランチが強制削除されます",
		},

		// ============================================================
		// Protect — block writes to sensitive files
		// ============================================================
		{
			Name:     "env-file",
			ToolName: "Write,Edit",
			Pattern:  `\.env`,
			Action:   "protect",
			Severity: "high",
			Message:  ".env ファイルへの書き込みを検出しました。機密情報が含まれている可能性があります",
		},
		{
			Name:     "pem-file",
			ToolName: "Write,Edit",
			Pattern:  `\.pem$`,
			Action:   "protect",
			Severity: "high",
			Message:  ".pem ファイルへの書き込みを検出しました。秘密鍵が含まれている可能性があります",
		},
		{
			Name:     "key-file",
			ToolName: "Write,Edit",
			Pattern:  `\.key$`,
			Action:   "protect",
			Severity: "high",
			Message:  ".key ファイルへの書き込みを検出しました。秘密鍵が含まれている可能性があります",
		},
		{
			Name:     "credentials",
			ToolName: "Write,Edit",
			Pattern:  `credentials`,
			Action:   "protect",
			Severity: "high",
			Message:  "credentials ファイルへの書き込みを検出しました",
		},
	}
}
