package store

import (
	"database/sql"
	"time"
)

// ClaudeSnapshot represents a stored Claude Code rate limit data point.
type ClaudeSnapshot struct {
	ID            int64
	FiveHourPct   float64
	SevenDayPct   *float64
	FiveHourReset *time.Time
	SevenDayReset *time.Time
	CapturedAt    time.Time
	Source        string // "statusline" or "manual"
}

// InsertClaudeSnapshot stores a Claude Code rate limit snapshot.
func (s *Store) InsertClaudeSnapshot(fiveHourPct float64, sevenDayPct *float64,
	fiveHourReset, sevenDayReset *time.Time, source string) (int64, error) {

	if source == "" {
		source = "statusline"
	}

	result, err := s.db.Exec(`
		INSERT INTO claude_snapshots (five_hour_pct, seven_day_pct, five_hour_reset, seven_day_reset, source)
		VALUES (?, ?, ?, ?, ?)`,
		fiveHourPct, sevenDayPct, fiveHourReset, sevenDayReset, source,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// LatestClaudeSnapshot returns the most recent Claude Code snapshot, or nil if none exist.
func (s *Store) LatestClaudeSnapshot() (*ClaudeSnapshot, error) {
	row := s.db.QueryRow(`
		SELECT id, five_hour_pct, seven_day_pct, five_hour_reset, seven_day_reset, captured_at, source
		FROM claude_snapshots
		ORDER BY captured_at DESC
		LIMIT 1`)

	snap := &ClaudeSnapshot{}
	var sevenPct sql.NullFloat64
	var fiveReset, sevenReset sql.NullTime

	err := row.Scan(&snap.ID, &snap.FiveHourPct, &sevenPct, &fiveReset, &sevenReset,
		&snap.CapturedAt, &snap.Source)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if sevenPct.Valid {
		snap.SevenDayPct = &sevenPct.Float64
	}
	if fiveReset.Valid {
		snap.FiveHourReset = &fiveReset.Time
	}
	if sevenReset.Valid {
		snap.SevenDayReset = &sevenReset.Time
	}

	return snap, nil
}

// ClaudeSnapshotHistory returns the last N Claude Code snapshots, newest first.
func (s *Store) ClaudeSnapshotHistory(limit int) ([]ClaudeSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, five_hour_pct, seven_day_pct, five_hour_reset, seven_day_reset, captured_at, source
		FROM claude_snapshots
		ORDER BY captured_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []ClaudeSnapshot
	for rows.Next() {
		snap := ClaudeSnapshot{}
		var sevenPct sql.NullFloat64
		var fiveReset, sevenReset sql.NullTime

		if err := rows.Scan(&snap.ID, &snap.FiveHourPct, &sevenPct, &fiveReset, &sevenReset,
			&snap.CapturedAt, &snap.Source); err != nil {
			return nil, err
		}

		if sevenPct.Valid {
			snap.SevenDayPct = &sevenPct.Float64
		}
		if fiveReset.Valid {
			snap.FiveHourReset = &fiveReset.Time
		}
		if sevenReset.Valid {
			snap.SevenDayReset = &sevenReset.Time
		}

		snaps = append(snaps, snap)
	}

	return snaps, rows.Err()
}
