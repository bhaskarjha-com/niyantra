# Data Model: Niyantra

## Database

**Engine:** SQLite 3 (via `modernc.org/sqlite`, pure Go)
**Location:** `~/.niyantra/niyantra.db` (auto-created)
**Encoding:** UTF-8, WAL mode

## Schema Versions

| Version | Tables | Description |
|---------|--------|-------------|
| v1 | `accounts`, `snapshots` | Core quota tracking |
| v2 | `subscriptions` | Manual subscription management |
| v3 | `config`, `activity_log`, `data_sources` + snapshot provenance | Infrastructure: config, audit trail, multi-source |
| v4 | `model_cycles` | Cycle intelligence — per-model reset detection and usage tracking |
| v5 | `claude_snapshots` + config keys | Claude Code rate limits, notifications, bridge config |
| v6 | `system_alerts` | System-level alerts with hybrid TTL, advisor integration |

---

## Tables

### `accounts`

Unique account identities, keyed by email address.

```sql
CREATE TABLE IF NOT EXISTS accounts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT    UNIQUE NOT NULL,
    plan_name  TEXT    DEFAULT '',
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);
```

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| `id` | INTEGER | PK, AUTO | Internal account ID |
| `email` | TEXT | UNIQUE, NOT NULL | Antigravity account email |
| `plan_name` | TEXT | DEFAULT '' | Latest known plan (Free, Pro, Enterprise) |
| `created_at` | DATETIME | DEFAULT now | First seen timestamp |
| `updated_at` | DATETIME | DEFAULT now | Last snapshot timestamp |

**Lifecycle:**
- Created via `GetOrCreateAccount(email)` on first snapshot
- `plan_name` and `updated_at` are refreshed on each snapshot
- Never deleted (historical record)

---

### `snapshots`

Point-in-time quota captures with full provenance.

```sql
CREATE TABLE IF NOT EXISTS snapshots (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL,
    captured_at     DATETIME NOT NULL,
    email           TEXT    NOT NULL,
    plan_name       TEXT    DEFAULT '',
    prompt_credits  REAL    DEFAULT 0,
    monthly_credits INTEGER DEFAULT 0,
    models_json     TEXT    NOT NULL,
    raw_json        TEXT    DEFAULT '',
    capture_method  TEXT    NOT NULL DEFAULT 'manual',  -- v3: manual, auto
    capture_source  TEXT    NOT NULL DEFAULT 'cli',     -- v3: cli, ui, watch, parser, import, mcp
    source_id       TEXT    NOT NULL DEFAULT 'antigravity', -- v3: FK → data_sources.id
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);

CREATE INDEX IF NOT EXISTS idx_snapshots_account_time
    ON snapshots(account_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_snapshots_time
    ON snapshots(captured_at DESC);
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | PK, AUTO |
| `account_id` | INTEGER | FK → accounts.id |
| `captured_at` | DATETIME | UTC timestamp of capture |
| `email` | TEXT | Email at time of capture |
| `plan_name` | TEXT | Plan name at time of capture |
| `prompt_credits` | REAL | Available prompt credits |
| `monthly_credits` | INTEGER | Monthly credit allocation |
| `models_json` | TEXT | JSON array of model quotas (see below) |
| `raw_json` | TEXT | Full API response for debugging |
| `capture_method` | TEXT | **v3:** `manual` or `auto` — was a human involved? |
| `capture_source` | TEXT | **v3:** `cli`, `ui`, `watch`, `parser`, `import`, `mcp` — which channel? |
| `source_id` | TEXT | **v3:** Which data source captured this (FK → data_sources.id) |

**Provenance tagging rules:**

| Trigger | `capture_method` | `capture_source` | `source_id` |
|---------|-----------------|------------------|-------------|
| `niyantra snap` (CLI) | manual | cli | antigravity |
| Dashboard "Snap Now" button | manual | ui | antigravity |
| Watch daemon polling | auto | watch | antigravity |
| Claude Code log parser | auto | parser | claude_code |
| Codex log parser | auto | parser | codex |
| CSV/JSON import | manual | import | (varies) |
| MCP agent trigger | auto | mcp | (varies) |

---

### `subscriptions` (v2)

Manually-tracked AI subscriptions — enriched with presets, trial tracking, and status page links.

```sql
CREATE TABLE IF NOT EXISTS subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    platform        TEXT    NOT NULL,
    category        TEXT    DEFAULT 'other',
    icon_key        TEXT    DEFAULT '',
    email           TEXT    DEFAULT '',
    plan_name       TEXT    DEFAULT '',
    status          TEXT    DEFAULT 'active',
    cost_amount     REAL    DEFAULT 0,
    cost_currency   TEXT    DEFAULT 'USD',
    billing_cycle   TEXT    DEFAULT 'monthly',
    token_limit     INTEGER DEFAULT 0,
    credit_limit    INTEGER DEFAULT 0,
    request_limit   INTEGER DEFAULT 0,
    limit_period    TEXT    DEFAULT 'monthly',
    limit_note      TEXT    DEFAULT '',
    next_renewal    TEXT    DEFAULT '',
    started_at      TEXT    DEFAULT '',
    trial_ends_at   TEXT    DEFAULT '',
    notes           TEXT    DEFAULT '',
    url             TEXT    DEFAULT '',
    status_page_url TEXT    DEFAULT '',
    auto_tracked    INTEGER DEFAULT 0,
    account_id      INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT (datetime('now')),
    updated_at      DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_renewal ON subscriptions(next_renewal);
CREATE INDEX IF NOT EXISTS idx_subscriptions_category ON subscriptions(category);
```

| Column | Type | Description |
|--------|------|-------------|
| `platform` | TEXT | Platform name (e.g., "Claude Pro", "Cursor Pro") |
| `category` | TEXT | coding, chat, api, image, audio, productivity, other |
| `status` | TEXT | active, trial, paused, cancelled |
| `cost_amount` | REAL | Cost per billing cycle |
| `billing_cycle` | TEXT | monthly, annual, lifetime, payg |
| `token_limit` / `credit_limit` / `request_limit` | INTEGER | 0 = unlimited |
| `limit_period` | TEXT | monthly, daily, weekly, rolling_3h, rolling_5h, hourly |
| `next_renewal` | TEXT | ISO date (YYYY-MM-DD) for next billing |
| `trial_ends_at` | TEXT | ISO date for trial expiry (shown as countdown badge) |
| `notes` | TEXT | Tips, benefits — pre-filled from presets |
| `url` | TEXT | Dashboard/billing URL (one-click access) |
| `status_page_url` | TEXT | Service status page URL |
| `auto_tracked` | INTEGER | 1 if auto-created by `snap`, 0 if manual |
| `account_id` | INTEGER | Links to `accounts.id` for auto-tracked entries |

---

### `config` (v3)

Server-level configuration stored as typed key-value pairs with metadata for auto-rendering in the Settings UI.

```sql
CREATE TABLE IF NOT EXISTS config (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    value_type  TEXT NOT NULL DEFAULT 'string',
    category    TEXT NOT NULL DEFAULT 'general',
    label       TEXT NOT NULL DEFAULT '',
    description TEXT DEFAULT '',
    updated_at  DATETIME DEFAULT (datetime('now'))
);
```

| Column | Type | Description |
|--------|------|-------------|
| `key` | TEXT | PK — setting identifier (e.g., `auto_capture`, `budget_monthly`) |
| `value` | TEXT | Setting value as string (parsed by `value_type`) |
| `value_type` | TEXT | `string`, `int`, `float`, `bool`, `json` — drives UI control type |
| `category` | TEXT | `capture`, `display`, `data`, `integration` — groups in Settings |
| `label` | TEXT | Human-readable label for the Settings UI |
| `description` | TEXT | Help text shown below the input |
| `updated_at` | DATETIME | Last modification timestamp |

**Default seeds:**

| Key | Value | Type | Category | Label |
|-----|-------|------|----------|-------|
| `auto_capture` | `false` | bool | capture | Auto Capture |
| `poll_interval` | `300` | int | capture | Poll Interval (s) |
| `auto_link_subs` | `true` | bool | capture | Auto-Link Subs |
| `budget_monthly` | `0` | float | display | Monthly Budget |
| `currency` | `USD` | string | display | Default Currency |
| `retention_days` | `365` | int | data | Retention (days) |
| `claude_bridge` | `false` | bool | integration | Claude Code Bridge |
| `notify_enabled` | `false` | bool | integration | Desktop Notifications |
| `notify_threshold` | `10` | int | integration | Notification Threshold (%) |

**Why metadata?** Adding a new config key in Go (e.g., `mcp_port` in Phase 7) automatically renders the correct UI control (number input, toggle, dropdown) without JavaScript changes.

---

### `data_sources` (v3)

Registry of available data sources with per-source configuration.

```sql
CREATE TABLE IF NOT EXISTS data_sources (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    source_type   TEXT NOT NULL,
    enabled       INTEGER NOT NULL DEFAULT 1,
    config_json   TEXT DEFAULT '{}',
    last_capture  DATETIME DEFAULT NULL,
    capture_count INTEGER DEFAULT 0,
    created_at    DATETIME DEFAULT (datetime('now'))
);
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | PK — source identifier (e.g., `antigravity`, `claude_code`) |
| `name` | TEXT | Human-readable name |
| `source_type` | TEXT | `ls_poll`, `log_parse`, `api_poll`, `manual` |
| `enabled` | INTEGER | 1 = active, 0 = disabled |
| `config_json` | TEXT | Source-specific config (paths, intervals, API keys) |
| `last_capture` | DATETIME | Timestamp of last successful capture |
| `capture_count` | INTEGER | Total captures from this source |
| `created_at` | DATETIME | When source was registered |

**Default seeds:**

| ID | Name | Type | Enabled | Config |
|----|------|------|---------|--------|
| `antigravity` | Antigravity | ls_poll | 1 | `{}` |
| `claude_code` | Claude Code | log_parse | 0 | `{"logPath":"~/.claude/projects"}` |
| `codex` | Codex | log_parse | 0 | `{"logPath":"~/.codex"}` |

**Source types:**

| Type | Capture Method | Description |
|------|---------------|-------------|
| `ls_poll` | Connect RPC to local language server | Antigravity |
| `log_parse` | Watch local JSONL files | Claude Code, Codex |
| `api_poll` | HTTP requests to cloud APIs | OpenAI, Anthropic (future) |
| `manual` | User enters data via UI | Manual usage logging |

---

### `activity_log` (v3)

Structured event log for audit trail, analytics, and notifications.

```sql
CREATE TABLE IF NOT EXISTS activity_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       DATETIME NOT NULL DEFAULT (datetime('now')),
    level           TEXT     NOT NULL DEFAULT 'info',
    source          TEXT     NOT NULL DEFAULT 'system',
    event_type      TEXT     NOT NULL,
    account_email   TEXT     DEFAULT '',
    snapshot_id     INTEGER  DEFAULT 0,
    details         TEXT     DEFAULT '{}',
    FOREIGN KEY (snapshot_id) REFERENCES snapshots(id)
);

CREATE INDEX IF NOT EXISTS idx_activity_log_time ON activity_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_activity_log_type ON activity_log(event_type);
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | PK, AUTO |
| `timestamp` | DATETIME | Event time (UTC) |
| `level` | TEXT | `debug`, `info`, `warn`, `error` |
| `source` | TEXT | `system`, `cli`, `ui`, `watch`, `parser`, `mcp` |
| `event_type` | TEXT | Event name (see taxonomy below) |
| `account_email` | TEXT | Associated account (if applicable) |
| `snapshot_id` | INTEGER | Associated snapshot (if applicable) |
| `details` | TEXT | JSON blob with event-specific data |

**Event taxonomy:**

| Event Type | When | Example Details |
|------------|------|-----------------|
| `server_start` | Server boots | `{"port":9222,"mode":"manual","version":"dev"}` |
| `snap` | Successful snapshot | `{"email":"...","plan":"Pro","method":"manual","source":"ui"}` |
| `snap_failed` | Snap attempt failed | `{"error":"LS not detected","source":"cli"}` |
| `config_change` | Setting updated | `{"key":"budget_monthly","from":"0","to":"200"}` |
| `sub_created` | Subscription created | `{"platform":"Claude Pro","auto":false}` |
| `sub_updated` | Subscription updated | `{"platform":"Claude Pro","field":"cost_amount"}` |
| `sub_deleted` | Subscription deleted | `{"platform":"Claude Pro","id":5}` |
| `auto_link` | Auto-created sub on snap | `{"email":"...","platform":"Antigravity"}` |
| `export` | CSV/JSON exported | `{"format":"csv","count":12}` |
| `watch_start` | Auto-poll started (future) | `{"interval":300,"source":"antigravity"}` |
| `watch_stop` | Auto-poll stopped (future) | `{"reason":"user_stopped"}` |
| `import` | Data imported (future) | `{"source":"claude_code","entries":15}` |

---

### `claude_snapshots` (v5)

Claude Code rate limit snapshots captured via the statusline bridge.

```sql
CREATE TABLE IF NOT EXISTS claude_snapshots (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    five_hour_pct   REAL NOT NULL,
    seven_day_pct   REAL,
    five_hour_reset DATETIME,
    seven_day_reset DATETIME,
    captured_at     DATETIME DEFAULT (datetime('now')),
    source          TEXT DEFAULT 'statusline'
);

CREATE INDEX IF NOT EXISTS idx_claude_snapshots_time
    ON claude_snapshots(captured_at DESC);
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | PK, AUTO |
| `five_hour_pct` | REAL | 5-hour window usage percentage (0-100) |
| `seven_day_pct` | REAL | 7-day window usage percentage (nullable) |
| `five_hour_reset` | DATETIME | When the 5-hour window resets |
| `seven_day_reset` | DATETIME | When the 7-day window resets |
| `captured_at` | DATETIME | Timestamp of capture |
| `source` | TEXT | `statusline` (bridge) or `manual` |

---

### `system_alerts` (v6)

System-level alerts for quota warnings, budget overages, and bridge errors. Uses a hybrid TTL strategy: critical/warning alerts persist until manually dismissed; info alerts auto-expire after 24 hours.

```sql
CREATE TABLE IF NOT EXISTS system_alerts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    severity   TEXT NOT NULL DEFAULT 'info',
    category   TEXT NOT NULL DEFAULT 'system',
    message    TEXT NOT NULL,
    dismissed  INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT (datetime('now')),
    expires_at DATETIME
);
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | PK, AUTO |
| `severity` | TEXT | `critical`, `warning`, or `info` |
| `category` | TEXT | Alert category: `quota`, `budget`, `bridge`, `system` |
| `message` | TEXT | Human-readable alert message |
| `dismissed` | INTEGER | 0 = active, 1 = dismissed |
| `created_at` | DATETIME | When the alert was created |
| `expires_at` | DATETIME | Auto-expiry time (NULL = never expires) |

**Hybrid TTL Rules:**
- `critical` / `warning`: `expires_at = NULL` — persist until dismissed
- `info`: `expires_at = created_at + 24h` — auto-cleaned by agent poll

---

## `models_json` Format

Each snapshot stores per-model quota data as a JSON array:

```json
[
  {
    "modelId": "MODEL_PLACEHOLDER_M35",
    "label": "Claude Sonnet 4.6 (Thinking)",
    "remainingFraction": 0.4,
    "remainingPercent": 40.0,
    "isExhausted": false,
    "resetTime": "2026-04-17T04:24:00Z"
  },
  {
    "modelId": "MODEL_PLACEHOLDER_M37",
    "label": "Gemini 3.1 Pro (High)",
    "remainingFraction": 1.0,
    "remainingPercent": 100.0,
    "isExhausted": false,
    "resetTime": "2026-04-17T04:48:00Z"
  }
]
```

---

## Key Queries

### Latest snapshot per account

Uses `MAX(id)` (not `MAX(captured_at)`) to guarantee exactly one row per account.

```sql
SELECT s.* FROM snapshots s
INNER JOIN (
    SELECT account_id, MAX(id) as max_id
    FROM snapshots
    GROUP BY account_id
) latest ON s.id = latest.max_id
ORDER BY s.captured_at DESC;
```

### Account get-or-create

```sql
INSERT INTO accounts (email, plan_name)
VALUES (?, ?)
ON CONFLICT(email) DO UPDATE SET
    plan_name = excluded.plan_name,
    updated_at = datetime('now');

SELECT id FROM accounts WHERE email = ?;
```

### Snapshot history with provenance

```sql
SELECT id, account_id, captured_at, email, plan_name,
       prompt_credits, monthly_credits, models_json,
       capture_method, capture_source, source_id
FROM snapshots
WHERE account_id = ?
ORDER BY captured_at DESC
LIMIT ?;
```

### Config read

```sql
SELECT value FROM config WHERE key = ?;
```

### Config read all (for Settings UI)

```sql
SELECT key, value, value_type, category, label, description
FROM config
ORDER BY category, key;
```

### Config update (with audit log)

```sql
UPDATE config SET value = ?, updated_at = datetime('now') WHERE key = ?;
```

### Recent activity

```sql
SELECT * FROM activity_log
ORDER BY timestamp DESC
LIMIT ?;
```

### Activity filtered by type

```sql
SELECT * FROM activity_log
WHERE event_type = ?
ORDER BY timestamp DESC
LIMIT ?;
```

### Data source status

```sql
SELECT id, name, source_type, enabled, last_capture, capture_count
FROM data_sources
ORDER BY id;
```

---

## Data Volume Estimates

Assuming 4 snaps/day across 3 accounts + 10 subscriptions + 20 activity events/day:

| Timeframe | Snapshots | Activity Events | Database Size |
|-----------|-----------|-----------------|---------------|
| 1 day | ~12 | ~20 | ~60 KB |
| 1 month | ~360 | ~600 | ~2 MB |
| 1 year | ~4,380 | ~7,300 | ~25 MB |
| 5 years | ~21,900 | ~36,500 | ~120 MB |

SQLite handles this trivially. Configurable retention (`config.retention_days`) allows auto-cleanup.

## Migration Strategy

Schema version is stored in SQLite's `user_version` pragma:

```sql
PRAGMA user_version;      -- read current version
PRAGMA user_version = 3;  -- current target (v3)
```

Migrations are embedded in Go code and run on startup:

```go
func (s *Store) migrate() error {
    version := s.getUserVersion()

    if version < 1 {
        // v1: accounts + snapshots tables
        s.exec(createAccountsSQL)
        s.exec(createSnapshotsSQL)
        s.setUserVersion(1)
    }

    if version < 2 {
        // v2: subscriptions table + indexes
        s.exec(createSubscriptionsSQL)
        s.setUserVersion(2)
    }

    if version < 3 {
        // v3: config, activity_log, data_sources, snapshot provenance
        s.exec(createConfigSQL)
        s.exec(createActivityLogSQL)
        s.exec(createDataSourcesSQL)
        s.exec(addSnapshotProvenanceSQL)
        s.exec(seedDefaultConfigSQL)
        s.exec(seedDefaultSourcesSQL)
        s.setUserVersion(3)
    }

    if version < 4 {
        // v4: model_cycles for cycle intelligence
        s.exec(createModelCyclesSQL)
        s.setUserVersion(4)
    }

    if version < 5 {
        // v5: Claude Code snapshots + notification/bridge config
        s.exec(createClaudeSnapshotsSQL)
        s.exec(seedPhase9ConfigSQL) // claude_bridge, notify_enabled, notify_threshold
        s.setUserVersion(5)
    }
}
```

**Backward compatibility:** All migrations are additive. Existing data is preserved. Existing snapshots get `capture_method='manual'` as the default, which is correct since all existing snapshots were manual captures.

## Client-Side Storage (localStorage)

Only **visual preferences** that have zero server impact stay in localStorage:

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `niyantra-theme` | enum | (absent) | `dark`, `light`, or absent (system preference) |

> **Note:** In v3, `budget` and `currency` moved from localStorage to the SQLite `config` table because they're needed server-side (CSV export headers, future MCP queries, CLI report formatting). A one-time JavaScript migration moves existing localStorage values to SQLite on first load after the v3 upgrade.

---

## Schema v7 — Phase 11: Codex & Sessions

### `codex_snapshots`

Stores Codex/ChatGPT usage snapshots with multi-quota support.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-incrementing ID |
| `account_id` | TEXT | OpenAI account UUID (from JWT `id_token`) |
| `captured_at` | DATETIME | UTC timestamp |
| `five_hour_pct` | REAL | 5-hour rolling window utilization (0-100) |
| `seven_day_pct` | REAL NULL | 7-day rolling window utilization |
| `code_review_pct` | REAL NULL | Code review quota utilization |
| `five_hour_reset` | DATETIME NULL | 5-hour window reset time |
| `seven_day_reset` | DATETIME NULL | 7-day window reset time |
| `plan_type` | TEXT | Plan tier (free, plus, pro, team) |
| `credits_balance` | REAL NULL | Remaining API credits |
| `capture_method` | TEXT | `manual` or `auto` |
| `capture_source` | TEXT | `ui` or `server` |

**Indexes:** `idx_codex_snap_account_time` on `(account_id, captured_at DESC)`

### `usage_sessions`

Tracks detected usage sessions across all providers.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-incrementing ID |
| `provider` | TEXT | `antigravity`, `codex`, or `claude` |
| `started_at` | DATETIME | Session start time |
| `ended_at` | DATETIME NULL | Session end time (NULL = active) |
| `duration_sec` | INTEGER | Duration in seconds (updated on close) |
| `snap_count` | INTEGER | Number of snapshots in this session |
| `created_at` | DATETIME | Record creation time |

**Indexes:** `idx_session_provider_time` on `(provider, started_at DESC)`

### `usage_logs`

Manual usage log entries linked to subscriptions.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-incrementing ID |
| `subscription_id` | INTEGER FK | Foreign key to subscriptions |
| `logged_at` | DATETIME | When usage was logged |
| `usage_amount` | REAL | Amount of usage |
| `usage_unit` | TEXT | Unit (requests, tokens, credits, minutes, hours, etc.) |
| `notes` | TEXT | Optional notes |

**Indexes:** `idx_usage_log_sub` on `(subscription_id, logged_at DESC)`

### New Config Keys (v7)

| Key | Default | Description |
|-----|---------|-------------|
| `codex_capture` | `false` | Enable Codex auto-polling |
| `session_idle_timeout` | `1200` | Seconds of idle before session closes |

