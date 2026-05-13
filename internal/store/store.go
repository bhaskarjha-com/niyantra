package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// isColumnExistsError returns true if the error is a SQLite "duplicate column"
// error, which is expected when ALTER TABLE ADD COLUMN runs on a column that
// was already added in a previous migration run.
func isColumnExistsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column")
}
// Store provides SQLite-backed persistence for Niyantra.
type Store struct {
	db   *sql.DB
	path string
}

// Open creates or opens a Niyantra database at the given path.
// Parent directories are created automatically.
func Open(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("store: creating directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("store: opening database: %w", err)
	}

	// Enable WAL mode for better concurrent read/write
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: setting WAL mode: %w", err)
	}

	// Set busy timeout to avoid SQLITE_BUSY during concurrent agent+UI writes
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: setting busy_timeout: %w", err)
	}

	s := &Store{db: db, path: dbPath}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migration failed: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

// migrate runs schema migrations based on user_version pragma.
func (s *Store) migrate() error {
	version := s.getUserVersion()

	if version < 1 {
		if _, err := s.db.Exec(`
			CREATE TABLE IF NOT EXISTS accounts (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				email      TEXT    UNIQUE NOT NULL,
				plan_name  TEXT    DEFAULT '',
				created_at DATETIME DEFAULT (datetime('now')),
				updated_at DATETIME DEFAULT (datetime('now'))
			);

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
		`); err != nil {
			return err
		}

		if err := s.setUserVersion(1); err != nil {
			return err
		}
	}

	if version < 2 {
		if _, err := s.db.Exec(`
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

			CREATE INDEX IF NOT EXISTS idx_subscriptions_status
				ON subscriptions(status);
			CREATE INDEX IF NOT EXISTS idx_subscriptions_renewal
				ON subscriptions(next_renewal);
			CREATE INDEX IF NOT EXISTS idx_subscriptions_category
				ON subscriptions(category);
		`); err != nil {
			return err
		}

		if err := s.setUserVersion(2); err != nil {
			return err
		}
	}

	if version < 3 {
		if _, err := s.db.Exec(`
			-- Server-level configuration (typed key-value)
			CREATE TABLE IF NOT EXISTS config (
				key         TEXT PRIMARY KEY,
				value       TEXT NOT NULL,
				value_type  TEXT NOT NULL DEFAULT 'string',
				category    TEXT NOT NULL DEFAULT 'general',
				label       TEXT NOT NULL DEFAULT '',
				description TEXT DEFAULT '',
				updated_at  DATETIME DEFAULT (datetime('now'))
			);

			-- Data sources registry
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

			-- Structured activity log
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

			-- Seed default config
			INSERT OR IGNORE INTO config (key, value, value_type, category, label, description) VALUES
				('auto_capture',   'false', 'bool',   'capture', 'Auto Capture',      'Enable autonomous data capture (polling, log parsing)'),
				('poll_interval',  '300',   'int',    'capture', 'Poll Interval (s)',  'Seconds between auto-polls when auto-capture is on'),
				('auto_link_subs', 'true',  'bool',   'capture', 'Auto-Link Subs',    'Auto-create subscription when new account detected on snap'),
				('budget_monthly', '0',     'float',  'display', 'Monthly Budget',     'Monthly AI spending budget ($)'),
				('currency',       'USD',   'string', 'display', 'Default Currency',   'Default currency for new subscriptions and reports'),
				('retention_days', '365',   'int',    'data',    'Retention (days)',   'How long to keep activity log and old snapshots');

			-- Seed default data sources
			INSERT OR IGNORE INTO data_sources (id, name, source_type, enabled, config_json) VALUES
				('antigravity', 'Antigravity', 'ls_poll',   1, '{}'),
				('claude_code', 'Claude Code', 'log_parse', 0, '{"logPath":"~/.claude/projects"}'),
				('codex',       'Codex',       'log_parse', 0, '{"logPath":"~/.codex"}');
		`); err != nil {
			return err
		}

		// Add provenance columns to snapshots (ALTER TABLE is separate — can't be in multi-statement)
		// S4: Log errors instead of swallowing them (duplicate column errors are expected on re-run)
		if _, err := s.db.Exec(`ALTER TABLE snapshots ADD COLUMN capture_method TEXT NOT NULL DEFAULT 'manual'`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter snapshots (capture_method): %w", err)
			}
		}
		if _, err := s.db.Exec(`ALTER TABLE snapshots ADD COLUMN capture_source TEXT NOT NULL DEFAULT 'cli'`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter snapshots (capture_source): %w", err)
			}
		}
		if _, err := s.db.Exec(`ALTER TABLE snapshots ADD COLUMN source_id TEXT NOT NULL DEFAULT 'antigravity'`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter snapshots (source_id): %w", err)
			}
		}

		if err := s.setUserVersion(3); err != nil {
			return err
		}
	}

	// ── v4: Reset cycle tracking ──────────────────────────────────
	if s.getUserVersion() < 4 {
		if _, err := s.db.Exec(`
			CREATE TABLE IF NOT EXISTS antigravity_reset_cycles (
				id             INTEGER PRIMARY KEY AUTOINCREMENT,
				model_id       TEXT     NOT NULL,
				account_id     INTEGER  NOT NULL DEFAULT 0,
				cycle_start    DATETIME NOT NULL,
				cycle_end      DATETIME,
				reset_time     DATETIME,
				peak_usage     REAL     NOT NULL DEFAULT 0,
				total_delta    REAL     NOT NULL DEFAULT 0,
				snapshot_count INTEGER  NOT NULL DEFAULT 0,
				FOREIGN KEY (account_id) REFERENCES accounts(id)
			);
			CREATE INDEX IF NOT EXISTS idx_cycles_model_start
				ON antigravity_reset_cycles(model_id, cycle_start);
			CREATE UNIQUE INDEX IF NOT EXISTS idx_cycles_active
				ON antigravity_reset_cycles(model_id, account_id)
				WHERE cycle_end IS NULL;
		`); err != nil {
			return err
		}

		if err := s.setUserVersion(4); err != nil {
			return err
		}
	}

	// ── v5: Claude Code snapshots + notifications config ──────────
	if s.getUserVersion() < 5 {
		if _, err := s.db.Exec(`
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

			-- Claude Code bridge + notification config
			INSERT OR IGNORE INTO config (key, value, value_type, category, label, description) VALUES
				('claude_bridge',     'false', 'bool',  'capture',      'Claude Code Bridge',   'Enable Claude Code statusline bridge for rate limit tracking'),
				('notify_enabled',    'false', 'bool',  'notification', 'Notifications',        'Enable desktop notifications for quota alerts'),
				('notify_threshold',  '10',    'float', 'notification', 'Alert Threshold (%)',   'Alert when model has less than this % remaining');

			-- Update Claude Code data source type
			UPDATE data_sources SET source_type = 'statusline_bridge' WHERE id = 'claude_code';
		`); err != nil {
			return err
		}

		if err := s.setUserVersion(5); err != nil {
			return err
		}
	}

	// ── v6: System alerts ────────────────────────────────────────────
	if s.getUserVersion() < 6 {
		if _, err := s.db.Exec(`
			CREATE TABLE IF NOT EXISTS system_alerts (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				alert_type   TEXT    NOT NULL,
				severity     TEXT    NOT NULL DEFAULT 'info',
				title        TEXT    NOT NULL,
				message      TEXT    NOT NULL,
				context_json TEXT    DEFAULT '{}',
				dismissed    INTEGER NOT NULL DEFAULT 0,
				created_at   DATETIME DEFAULT (datetime('now')),
				dismissed_at DATETIME,
				expires_at   DATETIME
			);
			CREATE INDEX IF NOT EXISTS idx_alerts_active
				ON system_alerts(dismissed, created_at DESC);
		`); err != nil {
			return err
		}

		if err := s.setUserVersion(6); err != nil {
			return err
		}
	}

	// ── v7: Codex snapshots + usage sessions + usage logs ──────────
	if s.getUserVersion() < 7 {
		if _, err := s.db.Exec(`
			CREATE TABLE IF NOT EXISTS codex_snapshots (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				account_id      TEXT    DEFAULT '',
				five_hour_pct   REAL    NOT NULL,
				seven_day_pct   REAL,
				code_review_pct REAL,
				five_hour_reset DATETIME,
				seven_day_reset DATETIME,
				plan_type       TEXT    DEFAULT '',
				credits_balance REAL,
				captured_at     DATETIME DEFAULT (datetime('now')),
				capture_method  TEXT    DEFAULT 'manual',
				capture_source  TEXT    DEFAULT 'ui'
			);
			CREATE INDEX IF NOT EXISTS idx_codex_snapshots_time
				ON codex_snapshots(captured_at DESC);
			CREATE INDEX IF NOT EXISTS idx_codex_snapshots_account
				ON codex_snapshots(account_id, captured_at DESC);

			CREATE TABLE IF NOT EXISTS usage_sessions (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				provider     TEXT    NOT NULL,
				started_at   DATETIME NOT NULL,
				ended_at     DATETIME,
				duration_sec INTEGER DEFAULT 0,
				snap_count   INTEGER DEFAULT 0,
				start_values TEXT    DEFAULT '[]',
				peak_values  TEXT    DEFAULT '[]',
				cost_hint    REAL,
				notes        TEXT    DEFAULT ''
			);
			CREATE INDEX IF NOT EXISTS idx_usage_sessions_provider
				ON usage_sessions(provider, started_at DESC);

			CREATE TABLE IF NOT EXISTS usage_logs (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				subscription_id INTEGER NOT NULL,
				logged_at       DATETIME DEFAULT (datetime('now')),
				usage_amount    REAL    NOT NULL,
				usage_unit      TEXT    NOT NULL,
				notes           TEXT    DEFAULT '',
				FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_usage_logs_sub
				ON usage_logs(subscription_id, logged_at DESC);

			-- Codex + session config keys
			INSERT OR IGNORE INTO config (key, value, value_type, category, label, description) VALUES
				('codex_capture',        'false', 'bool', 'capture', 'Codex Capture',          'Enable Codex quota tracking via OAuth API'),
				('session_idle_timeout', '1200',  'int',  'capture', 'Session Idle Timeout (s)', 'Seconds of inactivity before usage session ends (default 20 min)');

			-- Update Codex data source
			UPDATE data_sources SET source_type = 'oauth_api' WHERE id = 'codex';
		`); err != nil {
			return err
		}

		if err := s.setUserVersion(7); err != nil {
			return err
		}
	}

	// ── v8: AI Credits tracking ──────────────────────────────────────
	if s.getUserVersion() < 8 {
		if _, err := s.db.Exec(`ALTER TABLE snapshots ADD COLUMN ai_credits_json TEXT DEFAULT ''`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter snapshots (ai_credits_json): %w", err)
			}
		}
		if err := s.setUserVersion(8); err != nil {
			return err
		}
	}
	// ── v9: Codex email tracking ────────────────────────────────────
	if s.getUserVersion() < 9 {
		if _, err := s.db.Exec(`ALTER TABLE codex_snapshots ADD COLUMN email TEXT DEFAULT ''`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter codex_snapshots (email): %w", err)
			}
		}
		if err := s.setUserVersion(9); err != nil {
			return err
		}
	}

	// ── v10: Account notes, tags, pinned group (Phase 13: F1, F3) ────
	if s.getUserVersion() < 10 {
		if _, err := s.db.Exec(`ALTER TABLE accounts ADD COLUMN notes TEXT DEFAULT ''`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter accounts (notes): %w", err)
			}
		}
		if _, err := s.db.Exec(`ALTER TABLE accounts ADD COLUMN tags TEXT DEFAULT ''`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter accounts (tags): %w", err)
			}
		}
		if _, err := s.db.Exec(`ALTER TABLE accounts ADD COLUMN pinned_group TEXT DEFAULT ''`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter accounts (pinned_group): %w", err)
			}
		}
		if err := s.setUserVersion(10); err != nil {
			return err
		}
	}

	// ── v11: AI credit renewal day (per-account monthly renewal) ──
	if s.getUserVersion() < 11 {
		if _, err := s.db.Exec(`ALTER TABLE accounts ADD COLUMN credit_renewal_day INTEGER DEFAULT 0`); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("store: alter accounts (credit_renewal_day): %w", err)
			}
		}
		if err := s.setUserVersion(11); err != nil {
			return err
		}
	}

	// ── v12: Unified Account Model ──────────────────────────────────
	// Adds 'provider' column to accounts and migrates UNIQUE(email)
	// to UNIQUE(email, provider) so Codex/Claude/Cursor can each have
	// their own account row while sharing the metadata infrastructure
	// (tags, notes, pinned groups, tag-based filtering).
	// Also creates cursor_snapshots table for F15a.
	if s.getUserVersion() < 12 {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("store: v12 begin tx: %w", err)
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			}
		}()

		// 1. Recreate accounts table with UNIQUE(email, provider) instead of UNIQUE(email)
		if _, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS accounts_new (
				id                 INTEGER PRIMARY KEY AUTOINCREMENT,
				email              TEXT    NOT NULL,
				plan_name          TEXT    DEFAULT '',
				provider           TEXT    NOT NULL DEFAULT 'antigravity',
				notes              TEXT    DEFAULT '',
				tags               TEXT    DEFAULT '',
				pinned_group       TEXT    DEFAULT '',
				credit_renewal_day INTEGER DEFAULT 0,
				created_at         DATETIME DEFAULT (datetime('now')),
				updated_at         DATETIME DEFAULT (datetime('now')),
				UNIQUE(email, provider)
			);
		`); err != nil {
			return fmt.Errorf("store: v12 create accounts_new: %w", err)
		}

		// 2. Copy existing rows (all existing accounts are antigravity provider)
		if _, err = tx.Exec(`
			INSERT INTO accounts_new (id, email, plan_name, provider, notes, tags, pinned_group, credit_renewal_day, created_at, updated_at)
			SELECT id, email, plan_name, 'antigravity', COALESCE(notes,''), COALESCE(tags,''), COALESCE(pinned_group,''), COALESCE(credit_renewal_day,0), created_at, updated_at
			FROM accounts
		`); err != nil {
			return fmt.Errorf("store: v12 copy accounts: %w", err)
		}

		// 3. Drop old table, rename new
		if _, err = tx.Exec(`DROP TABLE accounts`); err != nil {
			return fmt.Errorf("store: v12 drop accounts: %w", err)
		}
		if _, err = tx.Exec(`ALTER TABLE accounts_new RENAME TO accounts`); err != nil {
			return fmt.Errorf("store: v12 rename accounts: %w", err)
		}

		// 4. Create cursor_snapshots table (F15a)
		if _, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS cursor_snapshots (
				id              INTEGER  PRIMARY KEY AUTOINCREMENT,
				account_id      INTEGER  DEFAULT 0,
				email           TEXT     DEFAULT '',
				premium_used    INTEGER  DEFAULT 0,
				premium_limit   INTEGER  DEFAULT 0,
				usage_pct       REAL     DEFAULT 0,
				plan_type       TEXT     DEFAULT '',
				start_of_month  TEXT     DEFAULT '',
				models_json     TEXT     DEFAULT '{}',
				captured_at     DATETIME DEFAULT (datetime('now')),
				capture_method  TEXT     DEFAULT 'manual',
				capture_source  TEXT     DEFAULT 'ui',
				FOREIGN KEY (account_id) REFERENCES accounts(id)
			);
			CREATE INDEX IF NOT EXISTS idx_cursor_snapshots_time
				ON cursor_snapshots(captured_at DESC);
			CREATE INDEX IF NOT EXISTS idx_cursor_snapshots_account
				ON cursor_snapshots(account_id, captured_at DESC);
		`); err != nil {
			return fmt.Errorf("store: v12 create cursor_snapshots: %w", err)
		}

		// 5. Seed Cursor config keys and data source
		if _, err = tx.Exec(`
			INSERT OR IGNORE INTO config (key, value, value_type, category, label, description) VALUES
				('cursor_capture',       'false', 'bool',   'capture', 'Cursor Capture',       'Enable Cursor quota tracking via session token API'),
				('cursor_session_token', '',      'string', 'capture', 'Cursor Session Token',  'Manual override: paste WorkosCursorSessionToken from browser DevTools');

			INSERT OR IGNORE INTO data_sources (id, name, source_type, enabled, config_json) VALUES
				('cursor', 'Cursor', 'session_token_api', 0, '{}');
		`); err != nil {
			return fmt.Errorf("store: v12 seed cursor config: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return fmt.Errorf("store: v12 commit: %w", err)
		}

		if err := s.setUserVersion(12); err != nil {
			return err
		}
	}

	// ── v13: Gemini CLI snapshots (F15b) ────────────────────────────
	if s.getUserVersion() < 13 {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("store: v13 begin tx: %w", err)
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			}
		}()

		// 1. Create gemini_snapshots table
		if _, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS gemini_snapshots (
				id              INTEGER  PRIMARY KEY AUTOINCREMENT,
				account_id      INTEGER  DEFAULT 0,
				email           TEXT     DEFAULT '',
				tier            TEXT     DEFAULT '',
				overall_pct     REAL     DEFAULT 0,
				models_json     TEXT     DEFAULT '[]',
				project_id      TEXT     DEFAULT '',
				captured_at     DATETIME DEFAULT (datetime('now')),
				capture_method  TEXT     DEFAULT 'manual',
				capture_source  TEXT     DEFAULT 'ui',
				FOREIGN KEY (account_id) REFERENCES accounts(id)
			);
			CREATE INDEX IF NOT EXISTS idx_gemini_snapshots_time
				ON gemini_snapshots(captured_at DESC);
			CREATE INDEX IF NOT EXISTS idx_gemini_snapshots_account
				ON gemini_snapshots(account_id, captured_at DESC);
		`); err != nil {
			return fmt.Errorf("store: v13 create gemini_snapshots: %w", err)
		}

		// 2. Seed Gemini config keys and data source
		if _, err = tx.Exec(`
			INSERT OR IGNORE INTO config (key, value, value_type, category, label, description) VALUES
				('gemini_capture',       'false', 'bool',   'capture', 'Gemini CLI Capture',      'Enable Gemini CLI quota tracking via OAuth API'),
				('gemini_client_id',     '',      'string', 'capture', 'Gemini OAuth Client ID',   'OAuth Client ID for token refresh (auto-detected from Gemini CLI installation)'),
				('gemini_client_secret', '',      'string', 'capture', 'Gemini OAuth Client Secret','OAuth Client Secret for token refresh (auto-detected from Gemini CLI installation)');

			INSERT OR IGNORE INTO data_sources (id, name, source_type, enabled, config_json) VALUES
				('gemini', 'Gemini CLI', 'oauth_api', 0, '{}');
		`); err != nil {
			return fmt.Errorf("store: v13 seed gemini config: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return fmt.Errorf("store: v13 commit: %w", err)
		}

		if err := s.setUserVersion(13); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) getUserVersion() int {
	var v int
	s.db.QueryRow("PRAGMA user_version").Scan(&v)
	return v
}

// SchemaVersion returns the current database schema version.
func (s *Store) SchemaVersion() int {
	return s.getUserVersion()
}

func (s *Store) setUserVersion(v int) error {
	_, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", v))
	if err != nil {
		return fmt.Errorf("store: setting schema version %d: %w", v, err)
	}
	return nil
}

// VacuumInto creates a consistent backup of the database at destPath.
// Uses VACUUM INTO which is WAL-safe and won't be corrupted by concurrent writes.
func (s *Store) VacuumInto(destPath string) error {
	_, err := s.db.Exec("VACUUM INTO ?", destPath)
	if err != nil {
		return fmt.Errorf("store: vacuum into %s: %w", destPath, err)
	}
	return nil
}
