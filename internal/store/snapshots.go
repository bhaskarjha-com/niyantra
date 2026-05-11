package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
)

// InsertSnapshot stores a snapshot in the database.
func (s *Store) InsertSnapshot(snap *client.Snapshot) (int64, error) {
	modelsJSON, err := json.Marshal(snap.Models)
	if err != nil {
		return 0, fmt.Errorf("store: marshal models: %w", err)
	}

	aiCreditsJSON := ""
	if len(snap.AICredits) > 0 {
		if b, err := json.Marshal(snap.AICredits); err == nil {
			aiCreditsJSON = string(b)
		}
	}

	result, err := s.db.Exec(`
		INSERT INTO snapshots (account_id, captured_at, email, plan_name,
			prompt_credits, monthly_credits, models_json, raw_json,
			capture_method, capture_source, source_id, ai_credits_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		snap.AccountID,
		snap.CapturedAt.UTC().Format(time.RFC3339),
		snap.Email,
		snap.PlanName,
		snap.PromptCredits,
		snap.MonthlyCredits,
		string(modelsJSON),
		snap.RawJSON,
		snap.CaptureMethod,
		snap.CaptureSource,
		snap.SourceID,
		aiCreditsJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("store: insert snapshot: %w", err)
	}

	return result.LastInsertId()
}

// LatestPerAccount returns the latest snapshot for each account.
func (s *Store) LatestPerAccount() ([]*client.Snapshot, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.account_id, s.captured_at, s.email, s.plan_name,
			s.prompt_credits, s.monthly_credits, s.models_json, s.raw_json,
			COALESCE(s.capture_method,'manual'), COALESCE(s.capture_source,'cli'), COALESCE(s.source_id,'antigravity'),
			COALESCE(s.ai_credits_json,'')
		FROM snapshots s
		INNER JOIN (
			SELECT account_id, MAX(id) as max_id
			FROM snapshots
			GROUP BY account_id
		) latest ON s.id = latest.max_id
		ORDER BY s.captured_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("store: query latest snapshots: %w", err)
	}
	defer rows.Close()

	return scanSnapshots(rows)
}

// History returns recent snapshots, optionally filtered by account.
func (s *Store) History(accountID int64, limit int) ([]*client.Snapshot, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []interface{}

	if accountID > 0 {
		query = `SELECT id, account_id, captured_at, email, plan_name,
			prompt_credits, monthly_credits, models_json, raw_json,
			COALESCE(capture_method,'manual'), COALESCE(capture_source,'cli'), COALESCE(source_id,'antigravity'),
			COALESCE(ai_credits_json,'')
			FROM snapshots WHERE account_id = ?
			ORDER BY captured_at DESC LIMIT ?`
		args = []interface{}{accountID, limit}
	} else {
		query = `SELECT id, account_id, captured_at, email, plan_name,
			prompt_credits, monthly_credits, models_json, raw_json,
			COALESCE(capture_method,'manual'), COALESCE(capture_source,'cli'), COALESCE(source_id,'antigravity'),
			COALESCE(ai_credits_json,'')
			FROM snapshots ORDER BY captured_at DESC LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query history: %w", err)
	}
	defer rows.Close()

	return scanSnapshots(rows)
}

// SnapshotCount returns the total number of snapshots.
func (s *Store) SnapshotCount() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM snapshots").Scan(&count)
	return count
}

// DeleteSnapshotsOlderThan removes snapshots older than the given number of days.
// Also cleans up old Claude and Codex snapshots with the same retention policy.
// Returns the total number of deleted rows.
func (s *Store) DeleteSnapshotsOlderThan(days int) (int64, error) {
	cutoff := fmt.Sprintf("-%d days", days)

	result, err := s.db.Exec(
		`DELETE FROM snapshots WHERE captured_at < datetime('now', ?)`, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("store: delete old snapshots: %w", err)
	}
	deleted, _ := result.RowsAffected()

	// Also clean up old Claude snapshots
	result2, err := s.db.Exec(
		`DELETE FROM claude_snapshots WHERE captured_at < datetime('now', ?)`, cutoff,
	)
	if err == nil {
		d2, _ := result2.RowsAffected()
		deleted += d2
	}

	// N13: Also clean up old Codex snapshots (previously unbounded)
	result3, err := s.db.Exec(
		`DELETE FROM codex_snapshots WHERE captured_at < datetime('now', ?)`, cutoff,
	)
	if err == nil {
		d3, _ := result3.RowsAffected()
		deleted += d3
	}

	return deleted, nil
}

// scanSnapshots reads snapshot rows into Snapshot structs.
func scanSnapshots(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
}) ([]*client.Snapshot, error) {
	var snapshots []*client.Snapshot

	type scanner interface {
		Next() bool
		Scan(dest ...interface{}) error
	}
	r := rows.(scanner)

	for r.Next() {
		var snap client.Snapshot
		var capturedAt string
		var modelsJSON string
		var aiCreditsJSON string

		if err := r.Scan(
			&snap.ID, &snap.AccountID, &capturedAt, &snap.Email,
			&snap.PlanName, &snap.PromptCredits, &snap.MonthlyCredits,
			&modelsJSON, &snap.RawJSON, &snap.CaptureMethod, &snap.CaptureSource, &snap.SourceID,
			&aiCreditsJSON,
		); err != nil {
			return nil, fmt.Errorf("store: scan snapshot: %w", err)
		}

		if t, err := time.Parse(time.RFC3339, capturedAt); err == nil {
			snap.CapturedAt = t
		}

		if err := json.Unmarshal([]byte(modelsJSON), &snap.Models); err != nil {
			snap.Models = nil // graceful degradation
		}

		if aiCreditsJSON != "" {
			json.Unmarshal([]byte(aiCreditsJSON), &snap.AICredits)
		}

		snapshots = append(snapshots, &snap)
	}

	return snapshots, nil
}

// UpdateSnapshotModels updates the models_json for a snapshot.
// Used by Quick Adjust to let users fine-tune quota percentages.
func (s *Store) UpdateSnapshotModels(snapshotID int64, models []client.ModelQuota) error {
	modelsJSON, err := json.Marshal(models)
	if err != nil {
		return fmt.Errorf("store: marshal models: %w", err)
	}

	result, err := s.db.Exec(
		`UPDATE snapshots SET models_json = ? WHERE id = ?`,
		string(modelsJSON), snapshotID,
	)
	if err != nil {
		return fmt.Errorf("store: update snapshot models: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("store: snapshot %d not found", snapshotID)
	}

	return nil
}

// RecentModelData is a lightweight snapshot for rate computation.
// Contains only the fields needed for burn rate calculation.
type RecentModelData struct {
	CapturedAt time.Time
	ModelsJSON string
}

// RecentModelSnapshots returns lightweight model data for an account
// captured within the given window. Results are ordered chronologically (ASC).
// Used by the forecast package for sliding-window rate computation.
func (s *Store) RecentModelSnapshots(accountID int64, window time.Duration) ([]RecentModelData, error) {
	since := time.Now().UTC().Add(-window).Format(time.RFC3339)

	rows, err := s.db.Query(`
		SELECT captured_at, models_json
		FROM snapshots
		WHERE account_id = ? AND captured_at >= ?
		ORDER BY captured_at ASC`,
		accountID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("store: recent model snapshots: %w", err)
	}
	defer rows.Close()

	var results []RecentModelData
	for rows.Next() {
		var capturedAtStr, modelsJSON string
		if err := rows.Scan(&capturedAtStr, &modelsJSON); err != nil {
			return nil, err
		}
		capturedAt, _ := time.Parse(time.RFC3339, capturedAtStr)
		results = append(results, RecentModelData{
			CapturedAt: capturedAt,
			ModelsJSON: modelsJSON,
		})
	}
	return results, rows.Err()
}

// RecentClaudeSnapshots returns recent Claude Code snapshots within the given
// window. Results are ordered chronologically (ASC).
// Used by the forecast package for Claude TTX computation.
func (s *Store) RecentClaudeSnapshots(window time.Duration) ([]ClaudeSnapshot, error) {
	since := time.Now().UTC().Add(-window).Format(time.RFC3339)

	return s.ClaudeSnapshotsSince(since)
}

// ClaudeSnapshotsSince returns Claude snapshots captured on or after the given
// RFC3339 timestamp, ordered chronologically (ASC).
func (s *Store) ClaudeSnapshotsSince(since string) ([]ClaudeSnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, five_hour_pct, seven_day_pct, five_hour_reset, seven_day_reset, captured_at, source
		FROM claude_snapshots
		WHERE captured_at >= ?
		ORDER BY captured_at ASC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanClaudeRows(rows)
}

// scanClaudeRows scans rows into ClaudeSnapshot structs.
func scanClaudeRows(rows *sql.Rows) ([]ClaudeSnapshot, error) {
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
	return snaps, nil
}

// RecentCodexSnapshots returns recent Codex snapshots within the given window,
// ordered chronologically (ASC). Used by the forecast package for Codex TTX.
func (s *Store) RecentCodexSnapshots(window time.Duration) ([]*CodexSnapshot, error) {
	since := time.Now().UTC().Add(-window)
	return s.CodexHistory(since)
}
