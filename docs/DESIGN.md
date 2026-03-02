# claude-pulse 設計書

## 背景と動機

### なぜ alfred から転換するのか

claude-alfred は Claude Code に特化した「静観型執事」として設計された。
しかし正直な評価（65/100）の結果、以下の問題が明確になった:

1. **Claude Code 自身が執事である** — Claude Code は既にユーザーの指示を理解し、ファイルを読み、コードを書く。alfred が「執事」を名乗ると「執事の執事」になってしまう
2. **機能重複が大きい** — Session Memory（ネイティブ）、auto memory（MEMORY.md）が alfred の decision 追跡と重複。knowledge 検索も Claude の知識と部分的に重複
3. **差別化ポイントが弱い** — co-changed files、hotspot、tool failure tracking は有用だが、それだけでは plugin としての存在意義が薄い

### 何に転換するのか

Claude Code エコシステムで**競合が存在しない 3 領域**に特化する:

| 柱 | 問い | 競合状況 |
|---|---|---|
| **Analytics** | どれだけ使っているか | なし — Claude Code にダッシュボード機能は存在しない |
| **Guardrails** | 安全に使えているか | なし — PreToolUse blocking hook を活用した安全機構は皆無 |
| **Coaching** | 上手く使えているか | 部分的 — Claude Code の知識はあるが、使用データに基づくパーソナライズド tips はない |

### ターゲットユーザー

ジュニア〜中堅エンジニア。Claude Code を使い始めたが、まだベストプラクティスを把握していないユーザー。

### 設計原則

- **コンテキスト圧迫ゼロ** — Hook は原則サイレント（PreToolUse のブロック時のみ出力）。MCP はオンデマンド。SessionStart の tip は有用な場合のみ
- **とりあえず導入してみよう** — install 一発で動作。設定不要。デフォルトプリセットで安全
- **コスト分析不要** — Max プランユーザー（$100/$200 固定）にとってトークンコストは無関係。代わりに「生産性の健全性」を計測

---

## アーキテクチャ

```
┌─────────────────────────────────────────────────┐
│              Claude Code Session                 │
│                                                  │
│  Hooks ──→ pulse.db                              │
│  SessionStart  (セッション記録)                    │
│  PreToolUse    (guardrail check → block/pass)    │
│  PostToolUse   (ツール使用記録)                    │
│  PostToolUseFailure (失敗記録)                    │
│  SessionEnd    (セッション終了)                    │
│                                                  │
│  MCP Tools (オンデマンド)                         │
│  stats  — 使用統計ダッシュボード                   │
│  guard  — guardrail ルール管理                    │
│  coach  — パーソナライズド tips + best practice   │
└─────────────────────────────────────────────────┘
```

### テクノロジースタック

- **Go 1.25** — シングルバイナリ、クロスコンパイル
- **SQLite** (ncruces/go-sqlite3) — 純 Go WASM ドライバ、CGO 不要
- **mcp-go** (mark3labs/mcp-go) — MCP サーバー SDK
- **Voyage AI** (optional) — voyage-4-large (1024d) embedding + rerank-2.5-lite

---

## DB スキーマ V1

```sql
-- セッション管理
CREATE TABLE sessions (
    id              TEXT PRIMARY KEY,
    project_path    TEXT NOT NULL,
    started_at      TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at        TEXT,
    tool_use_count  INTEGER NOT NULL DEFAULT 0,
    tool_fail_count INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ツール使用統計（1行 = 1ツール×1セッション）
CREATE TABLE tool_stats (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL,
    tool_name   TEXT NOT NULL,
    success     INTEGER NOT NULL DEFAULT 0,
    failure     INTEGER NOT NULL DEFAULT 0,
    last_used   TEXT NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    UNIQUE(session_id, tool_name)
);

-- Guardrail ルール定義
CREATE TABLE guardrail_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    tool_name   TEXT NOT NULL,        -- "Bash", "Write", "Edit", "*"
    pattern     TEXT NOT NULL,         -- regex pattern
    action      TEXT NOT NULL,         -- "block", "warn", "protect"
    severity    TEXT NOT NULL DEFAULT 'medium',  -- "critical", "high", "medium", "low"
    message     TEXT NOT NULL,
    enabled     INTEGER NOT NULL DEFAULT 1,
    is_preset   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Guardrail 発動ログ
CREATE TABLE guardrail_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL,
    rule_id     INTEGER NOT NULL,
    tool_name   TEXT NOT NULL,
    action      TEXT NOT NULL,
    matched     TEXT NOT NULL,         -- マッチした入力の抜粋
    timestamp   TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    FOREIGN KEY (rule_id) REFERENCES guardrail_rules(id)
);

-- ドキュメント知識ベース（coach 用）
CREATE TABLE docs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    url          TEXT NOT NULL,
    section_path TEXT NOT NULL,
    content      TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    source_type  TEXT NOT NULL,
    version      TEXT,
    crawled_at   TEXT NOT NULL,
    ttl_days     INTEGER DEFAULT 7,
    UNIQUE(url, section_path)
);

CREATE VIRTUAL TABLE docs_fts USING fts5(
    section_path, content,
    content='docs', content_rowid='id',
    tokenize='porter unicode61',
    prefix='2,3'
);

-- FTS5 同期トリガー（INSERT/UPDATE/DELETE）
-- (省略 — alfred の docs.go と同一パターン)

-- ベクトル埋め込み
CREATE TABLE embeddings (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    source     TEXT NOT NULL,
    source_id  INTEGER NOT NULL,
    model      TEXT NOT NULL,
    dims       INTEGER NOT NULL,
    vector     BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (source, source_id)
);

-- インデックス
CREATE INDEX idx_tool_stats_session ON tool_stats(session_id);
CREATE INDEX idx_tool_stats_name ON tool_stats(tool_name);
CREATE INDEX idx_guardrail_log_session ON guardrail_log(session_id);
CREATE INDEX idx_guardrail_log_timestamp ON guardrail_log(timestamp DESC);
CREATE INDEX idx_sessions_project ON sessions(project_path);
CREATE INDEX idx_sessions_started ON sessions(started_at DESC);
CREATE INDEX idx_docs_source_type ON docs(source_type);
CREATE INDEX idx_embeddings_source ON embeddings(source, source_id);
```

---

## Hook 設計（5 イベント）

### hookEvent 構造体

```go
type hookEvent struct {
    SessionID  string          `json:"session_id"`
    ProjectPath string         `json:"cwd"`
    ToolName   string          `json:"tool_name"`
    ToolInput  json.RawMessage `json:"tool_input,omitempty"`  // PreToolUse 用
    ToolError  bool            `json:"tool_error"`
    Source     string          `json:"source"`
}
```

### 1. SessionStart

- `st.EnsureSession(sessionID, projectPath)` でセッション記録
- 出力なし（サイレント）
- 将来拡張: 前回セッションの tip 注入（コンテキスト圧迫ゼロ原則に従い、有用な場合のみ）

### 2. PreToolUse（ブロッキング hook）

- stdin から toolName + toolInput を受信
- `guard.Engine.Check(toolName, toolInput)` で guardrail ルールとマッチング
- **block**: `{"decision":"block","reason":"⛔ [rule.message]"}` を stdout に出力 → Claude Code がツール実行をブロック
- **warn**: `{"decision":"block","reason":"⚠️ [rule.message] — ユーザーに確認してから再実行してください"}` を出力 → Claude が再試行可能
- **マッチなし**: 何も出力しない（サイレントパス）

### 3. PostToolUse

- `st.RecordToolUse(sessionID, toolName, true)` で成功を記録
- 出力なし

### 4. PostToolUseFailure

- `st.RecordToolUse(sessionID, toolName, false)` で失敗を記録
- 出力なし

### 5. SessionEnd

- `st.EndSession(sessionID)` で ended_at を更新
- 出力なし

---

## Guardrail エンジン

### guard.Engine

```go
type Engine struct {
    rules []Rule
}

type Rule struct {
    ID       int64
    Name     string
    ToolName string         // "Bash", "Write", "Edit", "*" (全ツール)
    Pattern  *regexp.Regexp
    Action   string         // "block", "warn", "protect"
    Severity string
    Message  string
}

type CheckResult struct {
    Action  string  // "block", "warn", "" (pass)
    Rule    *Rule
    Matched string  // マッチした入力テキストの抜粋
}

func (e *Engine) Check(toolName string, toolInput json.RawMessage) *CheckResult
```

### Check ロジック

1. toolInput を JSON パース → ツール別にチェック対象テキストを抽出
   - `Bash`: `.command` フィールド
   - `Write`: `.file_path` + `.content` フィールド
   - `Edit`: `.file_path` + `.old_string` + `.new_string` フィールド
   - その他: JSON 全体を文字列化
2. 該当ツール名（or `*`）のルールを順に適用
3. 最初にマッチしたルールの action を返す（block > warn の優先度）
4. マッチなし → nil を返す（サイレントパス）

### デフォルトプリセット

#### Block（即座にブロック — 危険コマンド）

| 名前 | ツール | パターン | メッセージ |
|---|---|---|---|
| rm-rf-root | Bash | `rm\s+-rf\s+/[^.]` | 危険: ルートディレクトリの再帰削除を検出 |
| fork-bomb | Bash | `:\(\)\s*\{\s*:\|:&\s*\}\s*;` | 危険: fork bomb を検出 |
| mkfs | Bash | `mkfs\.` | 危険: ファイルシステムフォーマットを検出 |
| dd-device | Bash | `dd\s+if=.*/dev/` | 危険: デバイスへの直接書き込みを検出 |
| dev-sda-write | Bash | `>\s*/dev/sd[a-z]` | 危険: ブロックデバイスへの書き込みを検出 |
| chmod-777-root | Bash | `chmod\s+-R\s+777\s+/` | 危険: ルートの権限を全開放する操作を検出 |

#### Warn（警告 — ユーザー確認を促す）

| 名前 | ツール | パターン | メッセージ |
|---|---|---|---|
| force-push | Bash | `git\s+push\s+.*--force` | git force push を検出。意図した操作か確認してください |
| hard-reset | Bash | `git\s+reset\s+--hard` | git hard reset を検出。未コミットの変更が失われます |
| drop-table | Bash | `DROP\s+TABLE` | DROP TABLE を検出。データベーステーブルが削除されます |
| truncate | Bash | `TRUNCATE\s+` | TRUNCATE を検出。テーブルの全データが削除されます |
| no-verify | Bash | `--no-verify` | --no-verify を検出。pre-commit hook がスキップされます |
| branch-delete-force | Bash | `git\s+branch\s+-D` | git branch -D を検出。ブランチが強制削除されます |

#### Protect（ファイル保護 — 機密ファイルの書き込みブロック）

| 名前 | ツール | パターン | メッセージ |
|---|---|---|---|
| env-file | Write,Edit | `\.env` (file_path に対して) | .env ファイルへの書き込みを検出。機密情報が含まれている可能性があります |
| pem-file | Write,Edit | `\.pem$` | .pem ファイルへの書き込みを検出。秘密鍵が含まれている可能性があります |
| key-file | Write,Edit | `\.key$` | .key ファイルへの書き込みを検出。秘密鍵が含まれている可能性があります |
| credentials | Write,Edit | `credentials` (file_path に対して) | credentials ファイルへの書き込みを検出 |

---

## MCP ツール設計（3 ツール）

### 1. stats — 使用統計ダッシュボード

```
入力:
  period: "today" | "week" | "month" | "all" (default: "week")
  project: string (optional, project path filter)

出力 (JSON):
{
  "period": "week",
  "sessions": {
    "total": 12,
    "avg_duration_min": 45
  },
  "tools": {
    "most_used": [
      {"name": "Edit", "count": 234, "success_rate": 0.95},
      {"name": "Bash", "count": 189, "success_rate": 0.87},
      ...
    ],
    "highest_failure": [
      {"name": "Bash", "failures": 24, "success_rate": 0.87},
      ...
    ]
  },
  "guardrails": {
    "total_blocked": 3,
    "total_warned": 7,
    "by_rule": [
      {"name": "force-push", "count": 4, "action": "warn"},
      ...
    ]
  }
}
```

### 2. guard — Guardrail ルール管理

```
入力:
  action: "list" | "add" | "remove" | "enable" | "disable" | "test" | "log"
  name: string (for add/remove/enable/disable)
  tool_name: string (for add)
  pattern: string (for add/test)
  rule_action: "block" | "warn" | "protect" (for add)
  message: string (for add)
  test_input: string (for test — テスト用入力)
  period: "today" | "week" | "month" | "all" (for log)

出力: action に応じた結果 JSON
```

### 3. coach — パーソナライズド tips + ベストプラクティス

```
入力:
  query: string (optional — ベストプラクティス検索)
  tips: boolean (default: false — 使用データからの tips 生成)

出力:
  query 指定時: ベストプラクティス検索結果（docs テーブル + ベクトル検索）
  tips=true: 使用データ分析に基づくパーソナライズドアドバイス
```

#### Tips 生成ロジック

使用統計から自動的にアドバイスを生成（LLM 不要、ルールベース）:

1. **Bash 過多パターン**: Bash の使用率が全体の 50% 以上 → 「Read/Edit/Grep 等の専用ツールを活用すると安全性と可読性が向上します」
2. **失敗率高パターン**: 特定ツールの失敗率が 30% 以上 → 「[ツール名] の失敗率が高いです。入力パラメータを確認してください」
3. **セッション長すぎ**: セッション内ツール使用が 200 超 → 「セッションが長くなっています。タスクを分割すると compact を避けられます」
4. **guardrail 頻発**: 同じルールが 3 回以上発動 → 「[ルール名] が繰り返し発動しています。ワークフローの見直しを検討してください」

---

## alfred からのコード移植マップ

| alfred ファイル | pulse 行き先 | 変更内容 |
|---|---|---|
| `internal/store/store.go` | `internal/store/store.go` | パス `~/.claude-pulse/pulse.db` に変更 |
| `internal/store/vectors.go` | `internal/store/vectors.go` | 変更なし |
| `internal/store/docs.go` | `internal/store/docs.go` | 変更なし |
| `internal/embedder/embedder.go` | `internal/embedder/embedder.go` | 変更なし |
| `internal/embedder/voyage.go` | `internal/embedder/voyage.go` | 変更なし |
| `internal/mcpserver/server.go` | `internal/mcpserver/server.go` | ツール3つ (stats/guard/coach) に変更 |
| `hooks.go` | `hooks.go` | 5イベント、ToolInput 追加、guard エンジン統合 |
| `main.go` | `main.go` | サブコマンド構成変更 |

**新規作成:**
- `internal/store/schema.go` — 新スキーマ V1
- `internal/store/tools.go` — ツール統計の CRUD
- `internal/store/guardrails.go` — Guardrail ルール・ログの CRUD
- `internal/guard/engine.go` — Guardrail マッチングエンジン
- `internal/guard/presets.go` — デフォルトプリセット定義
- `internal/analytics/stats.go` — 統計計算
- `internal/coach/tips.go` — Tips 生成（ルールベース）
- `internal/coach/knowledge.go` — ベストプラクティス検索
- `internal/mcpserver/handler_stats.go`
- `internal/mcpserver/handler_guard.go`
- `internal/mcpserver/handler_coach.go`

---

## パッケージ構成

```
claude-pulse/
├── main.go                          # CLI エントリ
├── hooks.go                         # Hook ハンドラ (5 イベント)
├── internal/
│   ├── store/
│   │   ├── store.go                 # SQLite 接続 + キャッシュ
│   │   ├── schema.go                # DDL + マイグレーション
│   │   ├── vectors.go               # ベクトル検索
│   │   ├── docs.go                  # docs テーブル CRUD + FTS5
│   │   ├── tools.go                 # tool_stats テーブル CRUD
│   │   └── guardrails.go            # guardrail_rules + guardrail_log CRUD
│   ├── embedder/
│   │   ├── embedder.go              # Embedder 公開 API
│   │   └── voyage.go                # Voyage AI HTTP クライアント
│   ├── guard/
│   │   ├── engine.go                # Guardrail マッチングエンジン
│   │   └── presets.go               # デフォルトプリセット
│   ├── analytics/
│   │   └── stats.go                 # 統計計算 + フォーマット
│   ├── coach/
│   │   ├── tips.go                  # ルールベース tips 生成
│   │   └── knowledge.go             # ベストプラクティス検索
│   ├── mcpserver/
│   │   ├── server.go                # MCP サーバー初期化
│   │   ├── handler_stats.go         # stats ツールハンドラ
│   │   ├── handler_guard.go         # guard ツールハンドラ
│   │   └── handler_coach.go         # coach ツールハンドラ
│   └── install/
│       └── install.go               # install/uninstall ロジック
├── plugin/
│   ├── plugin.json                  # Plugin メタデータ
│   ├── hooks/hooks.json             # Hook 登録（5 hooks）
│   ├── .mcp.json                    # MCP サーバー登録
│   └── bin/run.sh                   # Auto-download wrapper
├── docs/
│   └── DESIGN.md                    # この設計書
├── go.mod
├── go.sum
├── install.sh                       # curl インストーラ
├── .goreleaser.yaml
└── README.md
```

---

## テスト戦略

- **table-driven テスト** — Go 標準パターン
- **real SQLite** — `:memory:` ではなく `t.TempDir()` に DB を作成（WAL テスト含む）
- **t.Parallel()** — 独立テストは並列実行
- **Guard エンジン** — 全プリセットパターンのマッチ/非マッチテスト
- **Hook テスト** — stdin パイプで JSON を流し込み、stdout を検証
- **MCP ハンドラ** — JSON-RPC リクエスト/レスポンスの単体テスト

### 検証コマンド

```bash
go build -o pulse .                                                    # ビルド
go test ./...                                                          # 全テスト
go vet ./...                                                           # 静的解析
echo '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}' | \
  ./pulse hook PreToolUse                                              # guardrail テスト
```

---

## CLI コマンド

```
pulse - Your development health companion for Claude Code

Usage:
  pulse [command]

Commands:
  serve          MCP サーバー起動（stdio）
  hook <Event>   Hook ハンドラ（Claude Code から呼ばれる）
  seed-presets   デフォルト guardrail プリセットを DB に投入
  version        バージョン表示
  help           ヘルプ

Environment:
  VOYAGE_API_KEY     Optional. semantic vector search を有効化
  PULSE_DEBUG        デバッグログ有効化（~/.claude-pulse/debug.log）
```
