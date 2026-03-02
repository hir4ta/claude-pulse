package guard

import (
	"encoding/json"
	"testing"

	"github.com/hir4ta/claude-pulse/internal/store"
)

func TestEngineCheck_Block(t *testing.T) {
	t.Parallel()

	engine := NewEngine([]store.GuardrailRule{
		{ID: 1, Name: "rm-rf-root", ToolName: "Bash", Pattern: `rm\s+-rf\s+/[^.\s]`, Action: "block", Message: "blocked"},
	})

	tests := []struct {
		name      string
		toolName  string
		input     map[string]string
		wantBlock bool
	}{
		{"rm -rf /usr", "Bash", map[string]string{"command": "rm -rf /usr"}, true},
		{"rm -rf /home", "Bash", map[string]string{"command": "rm -rf /home"}, true},
		{"rm -rf ./node_modules", "Bash", map[string]string{"command": "rm -rf ./node_modules"}, false},
		{"safe command", "Bash", map[string]string{"command": "ls -la"}, false},
		{"wrong tool", "Edit", map[string]string{"command": "rm -rf /"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input, _ := json.Marshal(tt.input)
			result := engine.Check(tt.toolName, input)
			if tt.wantBlock && result == nil {
				t.Error("expected block, got nil")
			}
			if !tt.wantBlock && result != nil {
				t.Errorf("expected pass, got %+v", result)
			}
		})
	}
}

func TestEngineCheck_Warn(t *testing.T) {
	t.Parallel()

	engine := NewEngine([]store.GuardrailRule{
		{ID: 1, Name: "force-push", ToolName: "Bash", Pattern: `git\s+push\s+.*--force`, Action: "warn", Message: "warned"},
	})

	input, _ := json.Marshal(map[string]string{"command": "git push origin main --force"})
	result := engine.Check("Bash", input)
	if result == nil {
		t.Fatal("expected warn result")
	}
	if result.Action != "warn" {
		t.Errorf("action = %q, want warn", result.Action)
	}
}

func TestEngineCheck_Protect(t *testing.T) {
	t.Parallel()

	engine := NewEngine([]store.GuardrailRule{
		{ID: 1, Name: "env-file", ToolName: "Write,Edit", Pattern: `\.env`, Action: "protect", Message: "protected"},
	})

	tests := []struct {
		name        string
		toolName    string
		input       map[string]string
		wantProtect bool
	}{
		{"write .env", "Write", map[string]string{"file_path": "/app/.env"}, true},
		{"edit .env", "Edit", map[string]string{"file_path": "/app/.env.local"}, true},
		{"write normal", "Write", map[string]string{"file_path": "/app/main.go"}, false},
		{"bash .env", "Bash", map[string]string{"command": "cat .env"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input, _ := json.Marshal(tt.input)
			result := engine.Check(tt.toolName, input)
			if tt.wantProtect && result == nil {
				t.Error("expected protect, got nil")
			}
			if !tt.wantProtect && result != nil {
				t.Errorf("expected pass, got %+v", result)
			}
		})
	}
}

func TestEngineCheck_AllPresets(t *testing.T) {
	t.Parallel()

	engine := NewEngine(DefaultPresets())

	// Each preset should match its expected input.
	tests := []struct {
		name     string
		toolName string
		input    map[string]string
	}{
		{"rm-rf-root", "Bash", map[string]string{"command": "rm -rf /usr"}},
		{"fork-bomb", "Bash", map[string]string{"command": ":() { :|:& }; :"}},
		{"mkfs", "Bash", map[string]string{"command": "mkfs.ext4 /dev/sda1"}},
		{"dd-device", "Bash", map[string]string{"command": "dd if=/dev/zero of=/dev/sda"}},
		{"chmod-777-root", "Bash", map[string]string{"command": "chmod -R 777 /etc"}},
		{"force-push", "Bash", map[string]string{"command": "git push --force"}},
		{"hard-reset", "Bash", map[string]string{"command": "git reset --hard HEAD~1"}},
		{"drop-table", "Bash", map[string]string{"command": "sqlite3 db.sqlite 'DROP TABLE users'"}},
		{"no-verify", "Bash", map[string]string{"command": "git commit --no-verify -m 'skip'"}},
		{"env-file", "Write", map[string]string{"file_path": ".env"}},
		{"pem-file", "Edit", map[string]string{"file_path": "server.pem"}},
		{"key-file", "Write", map[string]string{"file_path": "private.key"}},
		{"credentials", "Edit", map[string]string{"file_path": "credentials.json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input, _ := json.Marshal(tt.input)
			result := engine.Check(tt.toolName, input)
			if result == nil {
				t.Errorf("preset %q should match", tt.name)
			}
		})
	}
}

func TestEngineCheck_EmptyInput(t *testing.T) {
	t.Parallel()

	engine := NewEngine(DefaultPresets())
	result := engine.Check("Bash", nil)
	if result != nil {
		t.Error("expected nil for empty input")
	}
}

func TestEngineCheck_InvalidRegex(t *testing.T) {
	t.Parallel()

	engine := NewEngine([]store.GuardrailRule{
		{ID: 1, Name: "bad-regex", ToolName: "Bash", Pattern: `[invalid`, Action: "block", Message: "bad"},
	})

	// Should have 0 compiled rules (invalid regex is skipped).
	input, _ := json.Marshal(map[string]string{"command": "anything"})
	result := engine.Check("Bash", input)
	if result != nil {
		t.Error("invalid regex rule should be skipped")
	}
}
