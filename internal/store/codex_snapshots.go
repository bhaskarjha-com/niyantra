package store

import (
	"database/sql"
	"time"
)

// CodexSnapshot represents a stored Codex usage snapshot.
// N14: AccountID is intentionally TEXT (OpenAI org ID like "org-xxx"),
// unlike Antigravity's INT64 which references the internal accounts table.
type CodexSnapshot struct {
	ID             int64      `json:"id"`
	AccountID      string     `json:"accountId"` // OpenAI org/account ID (TEXT, not FK to accounts table)
	FiveHourPct    float64    `json:"fiveHourPct"`
	SevenDayPct    *float64   `json:"sevenDayPct"`
	CodeReviewPct  *float64   `json:"codeReviewPct"`
	FiveHourReset  *time.Time `json:"fiveHourReset"`
	SevenDayReset  *time.Time `json:"sevenDayReset"`
	PlanType       string     `json:"planType"`
	CreditsBalance *float64   `json:"creditsBalance"`
	CapturedAt     time.Time  `json:"capturedAt"`
	CaptureMethod  string     `json:"captureMethod"`
	CaptureSource  string     `json:"captureSource"`
}

// InsertCodexSnapshot stores a Codex usage snapshot.
func (s *Store) InsertCodexSnapshot(snap *CodexSnapshot) (int64, error) {
	var fiveReset, sevenReset *string
	if snap.FiveHourReset != nil {
		t := snap.FiveHourReset.UTC().Format(time.RFC3339)
		fiveReset = &t
	}
	if snap.SevenDayReset != nil {
		t := snap.SevenDayReset.UTC().Format(time.RFC3339)
		sevenReset = &t
	}

	res, err := s.db.Exec(`
		INSERT INTO codex_snapshots
			(account_id, five_hour_pct, seven_day_pct, code_review_pct,
			 five_hour_reset, seven_day_reset, plan_type, credits_balance,
			 captured_at, capture_method, capture_source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), ?, ?)`,
		snap.AccountID, snap.FiveHourPct, snap.SevenDayPct, snap.CodeReviewPct,
		fiveReset, sevenReset, snap.PlanType, snap.CreditsBalance,
		snap.CaptureMethod, snap.CaptureSource,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LatestCodexSnapshot returns the most recent Codex snapshot.
func (s *Store) LatestCodexSnapshot() (*CodexSnapshot, error) {
	row := s.db.QueryRow(`
		SELECT id, account_id, five_hour_pct, seven_day_pct, code_review_pct,
		       five_hour_reset, seven_day_reset, plan_type, credits_balance,
		       captured_at, capture_method, capture_source
		FROM codex_snapshots ORDER BY captured_at DESC LIMIT 1`)

	snap := &CodexSnapshot{}
	var fiveReset, sevenReset, capturedAt sql.NullString
	err := row.Scan(
		&snap.ID, &snap.AccountID, &snap.FiveHourPct,
		&snap.SevenDayPct, &snap.CodeReviewPct,
		&fiveReset, &sevenReset,
		&snap.PlanType, &snap.CreditsBalance,
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
	if fiveReset.Valid {
		t, _ := time.Parse(time.RFC3339, fiveReset.String)
		snap.FiveHourReset = &t
	}
	if sevenReset.Valid {
		t, _ := time.Parse(time.RFC3339, sevenReset.String)
		snap.SevenDayReset = &t
	}

	return snap, nil
}

// CodexHistory returns Codex snapshots since the given time.
func (s *Store) CodexHistory(since time.Time) ([]*CodexSnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, account_id, five_hour_pct, seven_day_pct, code_review_pct,
		       five_hour_reset, seven_day_reset, plan_type, credits_balance,
		       captured_at, capture_method, capture_source
		FROM codex_snapshots
		WHERE captured_at >= ?
		ORDER BY captured_at ASC`,
		since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*CodexSnapshot
	for rows.Next() {
		snap := &CodexSnapshot{}
		var fiveReset, sevenReset, capturedAt sql.NullString
		if err := rows.Scan(
			&snap.ID, &snap.AccountID, &snap.FiveHourPct,
			&snap.SevenDayPct, &snap.CodeReviewPct,
			&fiveReset, &sevenReset,
			&snap.PlanType, &snap.CreditsBalance,
			&capturedAt, &snap.CaptureMethod, &snap.CaptureSource,
		); err != nil {
			return nil, err
		}
		if capturedAt.Valid {
			snap.CapturedAt, _ = time.Parse(time.RFC3339, capturedAt.String)
		}
		if fiveReset.Valid {
			t, _ := time.Parse(time.RFC3339, fiveReset.String)
			snap.FiveHourReset = &t
		}
		if sevenReset.Valid {
			t, _ := time.Parse(time.RFC3339, sevenReset.String)
			snap.SevenDayReset = &t
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}
