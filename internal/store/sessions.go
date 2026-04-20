package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// UsageSession represents a detected usage session across any provider.
type UsageSession struct {
	ID          int64      `json:"id"`
	Provider    string     `json:"provider"`
	StartedAt   time.Time  `json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt"`
	DurationSec int        `json:"durationSec"`
	SnapCount   int        `json:"snapCount"`
	StartValues string     `json:"startValues"` // JSON array
	PeakValues  string     `json:"peakValues"`  // JSON array
	CostHint    *float64   `json:"costHint"`
	Notes       string     `json:"notes"`
}

// CreateSession inserts a new usage session and returns its ID.
func (s *Store) CreateSession(provider string, startedAt time.Time, startValues []float64) (int64, error) {
	startJSON, _ := json.Marshal(startValues)
	res, err := s.db.Exec(`
		INSERT INTO usage_sessions (provider, started_at, start_values, peak_values)
		VALUES (?, ?, ?, ?)`,
		provider, startedAt.UTC().Format(time.RFC3339Nano),
		string(startJSON), string(startJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("store.CreateSession: %w", err)
	}
	return res.LastInsertId()
}

// CloseSession ends an active session with computed duration.
func (s *Store) CloseSession(id int64, endedAt time.Time) error {
	_, err := s.db.Exec(`
		UPDATE usage_sessions
		SET ended_at = ?,
		    duration_sec = CAST((julianday(?) - julianday(started_at)) * 86400 AS INTEGER)
		WHERE id = ?`,
		endedAt.UTC().Format(time.RFC3339Nano),
		endedAt.UTC().Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return fmt.Errorf("store.CloseSession: %w", err)
	}
	return nil
}

// IncrementSessionSnapCount increments the snapshot count for a session.
func (s *Store) IncrementSessionSnapCount(id int64) error {
	_, err := s.db.Exec(`UPDATE usage_sessions SET snap_count = snap_count + 1 WHERE id = ?`, id)
	return err
}

// UpdateSessionPeakValues updates peak values if current values exceed them.
func (s *Store) UpdateSessionPeakValues(id int64, values []float64) error {
	peakJSON, _ := json.Marshal(values)
	_, err := s.db.Exec(`UPDATE usage_sessions SET peak_values = ? WHERE id = ?`, string(peakJSON), id)
	return err
}

// ActiveSession returns the currently open session for a provider, or nil.
func (s *Store) ActiveSession(provider string) (*UsageSession, error) {
	row := s.db.QueryRow(`
		SELECT id, provider, started_at, ended_at, duration_sec, snap_count,
		       start_values, peak_values, cost_hint, notes
		FROM usage_sessions
		WHERE provider = ? AND ended_at IS NULL
		ORDER BY started_at DESC LIMIT 1`, provider)

	return scanSession(row)
}

// RecentSessions returns the most recent sessions, optionally filtered by provider.
func (s *Store) RecentSessions(provider string, limit int) ([]*UsageSession, error) {
	var query string
	var args []interface{}

	if provider != "" {
		query = `SELECT id, provider, started_at, ended_at, duration_sec, snap_count,
		                start_values, peak_values, cost_hint, notes
		         FROM usage_sessions WHERE provider = ?
		         ORDER BY started_at DESC LIMIT ?`
		args = []interface{}{provider, limit}
	} else {
		query = `SELECT id, provider, started_at, ended_at, duration_sec, snap_count,
		                start_values, peak_values, cost_hint, notes
		         FROM usage_sessions
		         ORDER BY started_at DESC LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store.RecentSessions: %w", err)
	}
	defer rows.Close()

	var sessions []*UsageSession
	for rows.Next() {
		sess, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// scanSession scans a single row into a UsageSession.
func scanSession(row *sql.Row) (*UsageSession, error) {
	sess := &UsageSession{}
	var startedAt string
	var endedAt sql.NullString
	var costHint sql.NullFloat64

	err := row.Scan(
		&sess.ID, &sess.Provider, &startedAt, &endedAt,
		&sess.DurationSec, &sess.SnapCount,
		&sess.StartValues, &sess.PeakValues,
		&costHint, &sess.Notes,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan session: %w", err)
	}

	sess.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	if endedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, endedAt.String)
		sess.EndedAt = &t
	}
	if costHint.Valid {
		sess.CostHint = &costHint.Float64
	}
	return sess, nil
}

// scanSessionRow scans a rows iterator into a UsageSession.
func scanSessionRow(rows *sql.Rows) (*UsageSession, error) {
	sess := &UsageSession{}
	var startedAt string
	var endedAt sql.NullString
	var costHint sql.NullFloat64

	err := rows.Scan(
		&sess.ID, &sess.Provider, &startedAt, &endedAt,
		&sess.DurationSec, &sess.SnapCount,
		&sess.StartValues, &sess.PeakValues,
		&costHint, &sess.Notes,
	)
	if err != nil {
		return nil, fmt.Errorf("store: scan session row: %w", err)
	}

	sess.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	if endedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, endedAt.String)
		sess.EndedAt = &t
	}
	if costHint.Valid {
		sess.CostHint = &costHint.Float64
	}
	return sess, nil
}
