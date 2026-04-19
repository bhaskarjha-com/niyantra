package store

import (
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

	result, err := s.db.Exec(`
		INSERT INTO snapshots (account_id, captured_at, email, plan_name,
			prompt_credits, monthly_credits, models_json, raw_json,
			capture_method, capture_source, source_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			s.prompt_credits, s.monthly_credits, s.models_json,
			COALESCE(s.capture_method,'manual'), COALESCE(s.capture_source,'cli'), COALESCE(s.source_id,'antigravity')
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
			prompt_credits, monthly_credits, models_json,
			COALESCE(capture_method,'manual'), COALESCE(capture_source,'cli'), COALESCE(source_id,'antigravity')
			FROM snapshots WHERE account_id = ?
			ORDER BY captured_at DESC LIMIT ?`
		args = []interface{}{accountID, limit}
	} else {
		query = `SELECT id, account_id, captured_at, email, plan_name,
			prompt_credits, monthly_credits, models_json,
			COALESCE(capture_method,'manual'), COALESCE(capture_source,'cli'), COALESCE(source_id,'antigravity')
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

		if err := r.Scan(
			&snap.ID, &snap.AccountID, &capturedAt, &snap.Email,
			&snap.PlanName, &snap.PromptCredits, &snap.MonthlyCredits,
			&modelsJSON, &snap.CaptureMethod, &snap.CaptureSource, &snap.SourceID,
		); err != nil {
			return nil, fmt.Errorf("store: scan snapshot: %w", err)
		}

		if t, err := time.Parse(time.RFC3339, capturedAt); err == nil {
			snap.CapturedAt = t
		}

		if err := json.Unmarshal([]byte(modelsJSON), &snap.Models); err != nil {
			snap.Models = nil // graceful degradation
		}

		snapshots = append(snapshots, &snap)
	}

	return snapshots, nil
}
