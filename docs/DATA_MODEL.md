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
    "modelId": "gpt-4o",
    "label": "GPT-4o",
    "remainingFraction": 0.4,
    "resetTime": "2026-04-17T04:24:00Z"
  },
  {
    "modelId": "claude-3.5-sonnet",
    "label": "Claude 3.5 Sonnet",
    "remainingFraction": 0.4,
    "resetTime": "2026-04-17T04:24:00Z"
  },
  {
    "modelId": "gemini-2.5-pro",
    "label": "Gemini 2.5 Pro",
    "remainingFraction": 1.0,
    "resetTime": "2026-04-17T04:48:00Z"
  },
  {
    "modelId": "gemini-2.5-flash",
    "label": "Gemini 2.5 Flash",
    "remainingFraction": 1.0,
    "resetTime": "2026-04-17T04:48:00Z"
  }
]
```

## Key Queries

### Latest snapshot per account

```sql
SELECT s.* FROM snapshots s
INNER JOIN (
    SELECT account_id, MAX(captured_at) as max_time
    FROM snapshots
    GROUP BY account_id
) latest ON s.account_id = latest.account_id
    AND s.captured_at = latest.max_time
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

## Data Volume Estimates

Assuming 4 snaps/day across 3 accounts:

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
PRAGMA user_version = 1;  -- set after migration
```

Migrations are embedded in Go code and run on startup:

```go
func (s *Store) migrate() error {
    version := s.getUserVersion()
    
    if version < 1 {
        // Initial schema
        s.exec(createAccountsSQL)
        s.exec(createSnapshotsSQL)
        s.setUserVersion(1)
    }
    
    // Future migrations:
    // if version < 2 { ... }
}
```
