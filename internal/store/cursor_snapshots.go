package store

import (
	"database/sql"
	"time"
)

// CursorSnapshot represents a stored Cursor usage snapshot.
type CursorSnapshot struct {
	ID            int64     `json:"id"`
	AccountID     int64     `json:"accountId"`
	Email         string    `json:"email,omitempty"`
	PremiumUsed   int       `json:"premiumUsed"`
	PremiumLimit  int       `json:"premiumLimit"`
	UsagePct      float64   `json:"usagePct"`
	PlanType      string    `json:"planType"`
	StartOfMonth  string    `json:"startOfMonth"`
	ModelsJSON    string    `json:"modelsJson"`
	CapturedAt    time.Time `json:"capturedAt"`
	CaptureMethod string    `json:"captureMethod"`
	CaptureSource string    `json:"captureSource"`
}

// InsertCursorSnapshot stores a Cursor usage snapshot.
func (s *Store) InsertCursorSnapshot(snap *CursorSnapshot) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO cursor_snapshots
			(account_id, email, premium_used, premium_limit, usage_pct,
			 plan_type, start_of_month, models_json,
			 captured_at, capture_method, capture_source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), ?, ?)`,
		snap.AccountID, snap.Email, snap.PremiumUsed, snap.PremiumLimit, snap.UsagePct,
		snap.PlanType, snap.StartOfMonth, snap.ModelsJSON,
		snap.CaptureMethod, snap.CaptureSource,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LatestCursorSnapshot returns the most recent Cursor snapshot.
func (s *Store) LatestCursorSnapshot() (*CursorSnapshot, error) {
	row := s.db.QueryRow(`
		SELECT id, account_id, COALESCE(email,''), premium_used, premium_limit, usage_pct,
		       COALESCE(plan_type,''), COALESCE(start_of_month,''), COALESCE(models_json,'{}'),
		       captured_at, capture_method, capture_source
		FROM cursor_snapshots ORDER BY captured_at DESC LIMIT 1`)

	snap := &CursorSnapshot{}
	var capturedAt sql.NullString
	err := row.Scan(
		&snap.ID, &snap.AccountID, &snap.Email,
		&snap.PremiumUsed, &snap.PremiumLimit, &snap.UsagePct,
		&snap.PlanType, &snap.StartOfMonth, &snap.ModelsJSON,
		&capturedAt, &snap.CaptureMethod, &snap.CaptureSource,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if capturedAt.Valid {
		snap.CapturedAt, _ = time.Parse(time.RFC3339, capturedAt.String)
	}

	return snap, nil
}

// RecentCursorSnapshots returns the last N cursor snapshots for heatmap/history.
func (s *Store) RecentCursorSnapshots(limit int) ([]*CursorSnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, account_id, COALESCE(email,''), premium_used, premium_limit, usage_pct,
		       COALESCE(plan_type,''), COALESCE(start_of_month,''), COALESCE(models_json,'{}'),
		       captured_at, capture_method, capture_source
		FROM cursor_snapshots
		ORDER BY captured_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*CursorSnapshot
	for rows.Next() {
		snap := &CursorSnapshot{}
		var capturedAt sql.NullString
		if err := rows.Scan(
			&snap.ID, &snap.AccountID, &snap.Email,
			&snap.PremiumUsed, &snap.PremiumLimit, &snap.UsagePct,
			&snap.PlanType, &snap.StartOfMonth, &snap.ModelsJSON,
			&capturedAt, &snap.CaptureMethod, &snap.CaptureSource,
		); err != nil {
			return nil, err
		}
		if capturedAt.Valid {
			snap.CapturedAt, _ = time.Parse(time.RFC3339, capturedAt.String)
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}
