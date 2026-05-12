package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CursorSnapshot represents a stored Cursor usage snapshot.
type CursorSnapshot struct {
	ID               int64     `json:"id"`
	AccountID        int64     `json:"accountId"`
	Email            string    `json:"email,omitempty"`
	BillingModel     string    `json:"billingModel"`     // request_count | usd_credit | unknown
	PlanTier         string    `json:"planTier"`          // free/pro/pro_plus/ultra/team
	RequestsUsed     int       `json:"requestsUsed"`      // legacy: numRequests
	RequestsMax      int       `json:"requestsMax"`       // legacy: maxRequestUsage
	UsedCents        int       `json:"usedCents"`         // credit: cents used
	LimitCents       int       `json:"limitCents"`        // credit: cents limit
	UsagePct         float64   `json:"usagePct"`          // overall usage %
	AutoPct          float64   `json:"autoPct"`           // auto+composer %
	APIPct           float64   `json:"apiPct"`            // API usage %
	CycleStart       string    `json:"cycleStart"`        // billing cycle start
	CycleEnd         string    `json:"cycleEnd"`          // billing cycle end
	CapturedAt       time.Time `json:"capturedAt"`
	CaptureMethod    string    `json:"captureMethod"`
	CaptureSource    string    `json:"captureSource"`
}

// InsertCursorSnapshot stores a Cursor usage snapshot.
func (s *Store) InsertCursorSnapshot(snap *CursorSnapshot) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO cursor_snapshots
			(account_id, email, premium_used, premium_limit, usage_pct,
			 plan_type, start_of_month, models_json,
			 captured_at, capture_method, capture_source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), ?, ?)`,
		snap.AccountID, snap.Email, snap.RequestsUsed, snap.RequestsMax, snap.UsagePct,
		snap.PlanTier, snap.CycleStart,
		formatCursorModelsJSON(snap),
		snap.CaptureMethod, snap.CaptureSource,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// formatCursorModelsJSON builds a JSON string with extended billing info.
func formatCursorModelsJSON(snap *CursorSnapshot) string {
	// Store billing details as models_json for frontend consumption
	return `{"billingModel":"` + snap.BillingModel + `",` +
		`"usedCents":` + itoa(snap.UsedCents) + `,` +
		`"limitCents":` + itoa(snap.LimitCents) + `,` +
		`"autoPct":` + ftoa(snap.AutoPct) + `,` +
		`"apiPct":` + ftoa(snap.APIPct) + `,` +
		`"cycleEnd":"` + snap.CycleEnd + `"}`
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func ftoa(f float64) string {
	return fmt.Sprintf("%.1f", f)
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
	var modelsJSON string
	err := row.Scan(
		&snap.ID, &snap.AccountID, &snap.Email,
		&snap.RequestsUsed, &snap.RequestsMax, &snap.UsagePct,
		&snap.PlanTier, &snap.CycleStart, &modelsJSON,
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
	parseCursorModelsJSON(snap, modelsJSON)

	return snap, nil
}

// parseCursorModelsJSON restores extended fields from the stored JSON.
func parseCursorModelsJSON(snap *CursorSnapshot, raw string) {
	var obj map[string]interface{}
	if json.Unmarshal([]byte(raw), &obj) != nil {
		return
	}
	if bm, ok := obj["billingModel"].(string); ok {
		snap.BillingModel = bm
	}
	if v, ok := obj["usedCents"].(float64); ok {
		snap.UsedCents = int(v)
	}
	if v, ok := obj["limitCents"].(float64); ok {
		snap.LimitCents = int(v)
	}
	if v, ok := obj["autoPct"].(float64); ok {
		snap.AutoPct = v
	}
	if v, ok := obj["apiPct"].(float64); ok {
		snap.APIPct = v
	}
	if v, ok := obj["cycleEnd"].(string); ok {
		snap.CycleEnd = v
	}
}

// RecentCursorSnapshots returns the last N cursor snapshots.
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
		var modelsJSON string
		if err := rows.Scan(
			&snap.ID, &snap.AccountID, &snap.Email,
			&snap.RequestsUsed, &snap.RequestsMax, &snap.UsagePct,
			&snap.PlanTier, &snap.CycleStart, &modelsJSON,
			&capturedAt, &snap.CaptureMethod, &snap.CaptureSource,
		); err != nil {
			return nil, err
		}
		if capturedAt.Valid {
			snap.CapturedAt, _ = time.Parse(time.RFC3339, capturedAt.String)
		}
		parseCursorModelsJSON(snap, modelsJSON)
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}
