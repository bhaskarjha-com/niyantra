# Data Model: Niyantra

## Database

**Engine:** SQLite 3 (via `modernc.org/sqlite`, pure Go)  
**Location:** `~/.niyantra/niyantra.db` (auto-created)  
**Encoding:** UTF-8, WAL mode

## Schema

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

Point-in-time quota captures.

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

---

## `models_json` Format

Each snapshot stores the per-model quota data as a JSON array:

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
    "modelId": "MODEL_PLACEHOLDER_M26",
    "label": "Claude Opus 4.6 (Thinking)",
    "remainingFraction": 0.4,
    "remainingPercent": 40.0,
    "isExhausted": false,
    "resetTime": "2026-04-17T04:24:00Z"
  },
  {
    "modelId": "MODEL_OPENAI_GPT_OSS_120B_MEDIUM",
    "label": "GPT-OSS 120B (Medium)",
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
  },
  {
    "modelId": "MODEL_PLACEHOLDER_M47",
    "label": "Gemini 3 Flash",
    "remainingFraction": 1.0,
    "remainingPercent": 100.0,
    "isExhausted": false,
    "resetTime": "2026-04-17T04:48:00Z"
  }
]
```

## Key Queries

### Latest snapshot per account

Uses `MAX(id)` (not `MAX(captured_at)`) to guarantee exactly one row per account, even if two snapshots land in the same second.

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

### Snapshot history for an account

```sql
SELECT * FROM snapshots
WHERE account_id = ?
ORDER BY captured_at DESC
LIMIT ?;
```

### Snapshot count

```sql
SELECT COUNT(*) FROM snapshots;
```

---

### `subscriptions` (Schema v2)

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

**Auto-Link:** When `niyantra snap` detects a new Antigravity account, it auto-creates a linked subscription record (auto_tracked=1) so it appears in the Subscriptions tab.

**Indexes:**
- `idx_subscriptions_status` — for filtering by status
- `idx_subscriptions_renewal` — for upcoming renewals queries
- `idx_subscriptions_category` — for category filtering

---

## Data Volume Estimates

Assuming 4 snaps/day across 3 accounts + 10 subscriptions:

| Timeframe | Snapshots | Database Size |
|-----------|-----------|---------------|
| 1 day | ~12 | ~50 KB |
| 1 month | ~360 | ~1.5 MB |
| 1 year | ~4,380 | ~18 MB |
| 5 years | ~21,900 | ~90 MB |

SQLite handles this trivially. No cleanup or rotation needed for years.

## Migration Strategy

Schema version is stored in SQLite's `user_version` pragma:

```sql
PRAGMA user_version;      -- read current version
PRAGMA user_version = 2;  -- current version (v2 adds subscriptions)
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
}
```

**Backward compatibility:** v2 migration is additive — existing v1 databases are upgraded seamlessly with no data loss.

## Client-Side Storage (localStorage)

Settings that don't need server persistence are stored in the browser's `localStorage`. These are per-browser and not shared between devices.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `niyantra-budget` | float string | (absent) | Monthly AI spending budget in base currency |
| `niyantra-currency` | ISO 4217 | `USD` | Default currency for new subscriptions |
| `niyantra-theme` | enum | (absent) | `dark`, `light`, or absent (system preference) |

**Design rationale:** Budget/currency/theme are presentation-layer settings with no value in the SQLite database. Keeping them in `localStorage` means:
- Zero schema changes needed
- Settings survive database resets
- No API calls for settings reads/writes
- Instant UI response (no network round-trip)
