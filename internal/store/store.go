package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

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

		s.setUserVersion(1)
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

		s.setUserVersion(2)
	}

	return nil
}

func (s *Store) getUserVersion() int {
	var v int
	s.db.QueryRow("PRAGMA user_version").Scan(&v)
	return v
}

func (s *Store) setUserVersion(v int) {
	s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", v))
}
