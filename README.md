# pulse

Your development health companion for Claude Code.

Pulse watches your coding sessions quietly — tracking tool usage, blocking
dangerous commands, and coaching you toward better practices. Zero context
window overhead by default.

## What Pulse Does

**Analytics** — See how you use Claude Code. Tool success rates, session
trends, and usage patterns at a glance.

**Guardrails** — Stay safe automatically. Pulse blocks destructive commands
(`rm -rf /`, fork bombs), warns on risky operations (force push, hard reset),
and protects sensitive files (.env, .pem, .key).

**Coaching** — Get better at Claude Code. Personalized tips based on your
actual usage data, plus searchable best practices.

## Install

```bash
go install github.com/hir4ta/claude-pulse@latest
```

バイナリは `$GOPATH/bin/pulse` にインストールされます。
`$GOPATH/bin` が PATH に含まれていることを確認してください。

Guardrail プリセットは MCP サーバー起動時に自動 seed されます。

**API key** (optional):

```bash
export VOYAGE_API_KEY=your-key       # Semantic search (Voyage AI voyage-4-large)
```

Without `VOYAGE_API_KEY`, coach search falls back to FTS5 keyword search.

## MCP Tools (3)

| Tool | What it does |
|------|-------------|
| `stats` | Usage statistics dashboard: tool success rates, session trends, guardrail activity |
| `guard` | Manage guardrail rules: list, add, remove, enable/disable, test patterns, view log |
| `coach` | Personalized tips from usage data + Claude Code best practice search |

## Hooks (5)

All hooks are silent except PreToolUse (which blocks dangerous commands).

| Hook | When | What it does |
|------|------|-------------|
| `SessionStart` | Session begins | Record session, seed default guardrail presets |
| `PreToolUse` | Before tool execution | Check guardrails — block/warn/protect or silent pass |
| `PostToolUse` | After tool succeeds | Record tool success (silent) |
| `PostToolUseFailure` | After tool fails | Record tool failure (silent) |
| `SessionEnd` | Session closes | Finalize session (silent) |

## Default Guardrail Presets

### Block (immediately prevent)
- `rm -rf /` — recursive root deletion
- Fork bombs, `mkfs`, `dd` to devices, `chmod -R 777 /`

### Warn (prompt for confirmation)
- `git push --force`, `git reset --hard`
- `DROP TABLE`, `TRUNCATE`, `--no-verify`, `git branch -D`

### Protect (block writes to sensitive files)
- `.env`, `.pem`, `.key`, `credentials`

## How It Works

```
┌─────────────────────────────────────────┐
│        Your Claude Code Session          │
│                                          │
│  Hooks ──→ pulse.db                      │
│  SessionStart    (record session)        │
│  PreToolUse      (guardrail check)       │
│  PostToolUse     (record success)        │
│  PostToolFailure (record failure)        │
│  SessionEnd      (finalize)              │
│                                          │
│  MCP Tools (on-demand, zero overhead)    │
│  stats  — usage dashboard                │
│  guard  — rule management                │
│  coach  — tips + best practices          │
└─────────────────────────────────────────┘
```

## Debug

Set `PULSE_DEBUG=1` to enable debug logging to `~/.claude-pulse/debug.log`.

## Building from source

```bash
git clone https://github.com/hir4ta/claude-pulse
cd claude-pulse
go build -o pulse .
```

## Dependencies

| Library | Purpose |
|---------|---------|
| [mcp-go](https://github.com/mark3labs/mcp-go) | MCP server SDK |
| [go-sqlite3](https://github.com/ncruces/go-sqlite3) | SQLite driver (pure Go, WASM) |

## License

MIT
