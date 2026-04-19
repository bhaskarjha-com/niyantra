package store

import (
	"fmt"
	"time"
)

// Cycle represents a single reset cycle for an Antigravity model.
type Cycle struct {
	ID            int64      `json:"id"`
	ModelID       string     `json:"modelId"`
	AccountID     int64      `json:"accountId"`
	CycleStart    time.Time  `json:"cycleStart"`
	CycleEnd      *time.Time `json:"cycleEnd"`
	ResetTime     *time.Time `json:"resetTime"`
	PeakUsage     float64    `json:"peakUsage"`
	TotalDelta    float64    `json:"totalDelta"`
	SnapshotCount int        `json:"snapshotCount"`
}

// CreateCycle inserts a new active cycle for the given model+account.
func (s *Store) CreateCycle(modelID string, accountID int64, start time.Time, resetTime *time.Time) (int64, error) {
	var rt interface{}
	if resetTime != nil {
		rt = resetTime.UTC().Format(time.RFC3339)
	}

	res, err := s.db.Exec(`
		INSERT INTO antigravity_reset_cycles (model_id, account_id, cycle_start, reset_time)
		VALUES (?, ?, ?, ?)
	`, modelID, accountID, start.UTC().Format(time.RFC3339), rt)
	if err != nil {
		return 0, fmt.Errorf("store: create cycle %s: %w", modelID, err)
	}
	return res.LastInsertId()
}

// CloseCycle ends the active cycle for a model+account.
func (s *Store) CloseCycle(modelID string, accountID int64, endTime time.Time, peakUsage, totalDelta float64) error {
	_, err := s.db.Exec(`
		UPDATE antigravity_reset_cycles
		SET cycle_end = ?, peak_usage = ?, total_delta = ?
		WHERE model_id = ? AND account_id = ? AND cycle_end IS NULL
	`, endTime.UTC().Format(time.RFC3339), peakUsage, totalDelta, modelID, accountID)
	if err != nil {
		return fmt.Errorf("store: close cycle %s: %w", modelID, err)
	}
	return nil
}

// UpdateCycle updates peak_usage and total_delta for the active cycle.
func (s *Store) UpdateCycle(modelID string, accountID int64, peakUsage, totalDelta float64, snapshotCount int) error {
	_, err := s.db.Exec(`
		UPDATE antigravity_reset_cycles
		SET peak_usage = ?, total_delta = ?, snapshot_count = ?
		WHERE model_id = ? AND account_id = ? AND cycle_end IS NULL
	`, peakUsage, totalDelta, snapshotCount, modelID, accountID)
	if err != nil {
		return fmt.Errorf("store: update cycle %s: %w", modelID, err)
	}
	return nil
}

// ActiveCycle returns the currently active (unclosed) cycle for a model+account, or nil.
func (s *Store) ActiveCycle(modelID string, accountID int64) (*Cycle, error) {
	c := &Cycle{}
	var startStr string
	var endStr, resetStr *string

	err := s.db.QueryRow(`
		SELECT id, model_id, account_id, cycle_start, cycle_end, reset_time,
		       peak_usage, total_delta, snapshot_count
		FROM antigravity_reset_cycles
		WHERE model_id = ? AND account_id = ? AND cycle_end IS NULL
	`, modelID, accountID).Scan(
		&c.ID, &c.ModelID, &c.AccountID, &startStr, &endStr, &resetStr,
		&c.PeakUsage, &c.TotalDelta, &c.SnapshotCount,
	)
	if err != nil {
		return nil, nil // No active cycle
	}

	c.CycleStart, _ = time.Parse(time.RFC3339, startStr)
	if resetStr != nil {
		t, _ := time.Parse(time.RFC3339, *resetStr)
		c.ResetTime = &t
	}
	return c, nil
}

// CycleHistory returns the most recent closed cycles for a model+account.
func (s *Store) CycleHistory(modelID string, accountID int64, limit int) ([]*Cycle, error) {
	rows, err := s.db.Query(`
		SELECT id, model_id, account_id, cycle_start, cycle_end, reset_time,
		       peak_usage, total_delta, snapshot_count
		FROM antigravity_reset_cycles
		WHERE model_id = ? AND account_id = ? AND cycle_end IS NOT NULL
		ORDER BY cycle_start DESC
		LIMIT ?
	`, modelID, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: cycle history %s: %w", modelID, err)
	}
	defer rows.Close()

	var cycles []*Cycle
	for rows.Next() {
		c := &Cycle{}
		var startStr string
		var endStr, resetStr *string
		if err := rows.Scan(
			&c.ID, &c.ModelID, &c.AccountID, &startStr, &endStr, &resetStr,
			&c.PeakUsage, &c.TotalDelta, &c.SnapshotCount,
		); err != nil {
			return nil, err
		}
		c.CycleStart, _ = time.Parse(time.RFC3339, startStr)
		if endStr != nil {
			t, _ := time.Parse(time.RFC3339, *endStr)
			c.CycleEnd = &t
		}
		if resetStr != nil {
			t, _ := time.Parse(time.RFC3339, *resetStr)
			c.ResetTime = &t
		}
		cycles = append(cycles, c)
	}
	return cycles, nil
}

// AllActiveCycles returns all active cycles for a given account.
func (s *Store) AllActiveCycles(accountID int64) ([]*Cycle, error) {
	rows, err := s.db.Query(`
		SELECT id, model_id, account_id, cycle_start, cycle_end, reset_time,
		       peak_usage, total_delta, snapshot_count
		FROM antigravity_reset_cycles
		WHERE account_id = ? AND cycle_end IS NULL
		ORDER BY model_id
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("store: all active cycles: %w", err)
	}
	defer rows.Close()

	var cycles []*Cycle
	for rows.Next() {
		c := &Cycle{}
		var startStr string
		var endStr, resetStr *string
		if err := rows.Scan(
			&c.ID, &c.ModelID, &c.AccountID, &startStr, &endStr, &resetStr,
			&c.PeakUsage, &c.TotalDelta, &c.SnapshotCount,
		); err != nil {
			return nil, err
		}
		c.CycleStart, _ = time.Parse(time.RFC3339, startStr)
		if resetStr != nil {
			t, _ := time.Parse(time.RFC3339, *resetStr)
			c.ResetTime = &t
		}
		cycles = append(cycles, c)
	}
	return cycles, nil
}
