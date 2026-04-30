package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// ImportResult summarizes what happened during a JSON import.
type ImportResult struct {
	AccountsCreated   int      `json:"accountsCreated"`
	AccountsSkipped   int      `json:"accountsSkipped"`
	SubsCreated       int      `json:"subsCreated"`
	SubsSkipped       int      `json:"subsSkipped"`
	SnapshotsImported int      `json:"snapshotsImported"`
	SnapshotsDuped    int      `json:"snapshotsDuped"`
	Errors            []string `json:"errors"`
}

// importEnvelope is the expected JSON export structure.
type importEnvelope struct {
	Version       string            `json:"version"`
	ExportedAt    string            `json:"exportedAt"`
	Accounts      []json.RawMessage `json:"accounts"`
	Subscriptions []json.RawMessage `json:"subscriptions"`
	Snapshots     []json.RawMessage `json:"snapshots"`
}

type importAccount struct {
	Email    string `json:"email"`
	PlanName string `json:"plan_name"`
}

type importSubscription struct {
	Platform     string  `json:"platform"`
	Email        string  `json:"email"`
	Category     string  `json:"category"`
	PlanName     string  `json:"plan_name"`
	Status       string  `json:"status"`
	CostAmount   float64 `json:"cost_amount"`
	CostCurrency string  `json:"cost_currency"`
	BillingCycle string  `json:"billing_cycle"`
	NextRenewal  string  `json:"next_renewal"`
	Notes        string  `json:"notes"`
}

type importSnapshot struct {
	AccountID  int64  `json:"account_id"`
	Email      string `json:"email"`
	CapturedAt string `json:"captured_at"`
	ModelsJSON string `json:"models_json"`
	PlanName   string `json:"plan_name"`
}

// ImportJSON imports data from a JSON export, using additive merge with deduplication.
// Accounts are deduped by email. Subscriptions by platform+email.
// Snapshots by account+captured_at (1-second window). Config is never overwritten.
func (s *Store) ImportJSON(data []byte) (*ImportResult, error) {
	result := &ImportResult{}

	var envelope importEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("import: invalid JSON: %w", err)
	}

	// N11: Accept both "1.0" (from our export) and "niyantra-export-v1" (legacy)
	if envelope.Version != "" && envelope.Version != "1.0" && envelope.Version != "niyantra-export-v1" {
		return nil, fmt.Errorf("import: unsupported export version %q (expected 1.0 or niyantra-export-v1)", envelope.Version)
	}

	// ── Import accounts (dedup by email) ──
	emailToID := make(map[string]int64)
	for _, raw := range envelope.Accounts {
		var acct importAccount
		if err := json.Unmarshal(raw, &acct); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("account parse: %v", err))
			continue
		}
		if acct.Email == "" {
			continue
		}

		// Check if exists
		var existingID int64
		err := s.db.QueryRow(`SELECT id FROM accounts WHERE email = ?`, acct.Email).Scan(&existingID)
		if err == nil {
			emailToID[acct.Email] = existingID
			result.AccountsSkipped++
			continue
		}

		res, err := s.db.Exec(`INSERT INTO accounts (email, plan_name) VALUES (?, ?)`,
			acct.Email, acct.PlanName)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("account insert %q: %v", acct.Email, err))
			continue
		}
		id, _ := res.LastInsertId()
		emailToID[acct.Email] = id
		result.AccountsCreated++
	}

	// ── Import subscriptions (dedup by platform+email) ──
	for _, raw := range envelope.Subscriptions {
		var sub importSubscription
		if err := json.Unmarshal(raw, &sub); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("subscription parse: %v", err))
			continue
		}
		if sub.Platform == "" {
			continue
		}

		// Check if exists (platform + email combo)
		var count int
		s.db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE platform = ? AND email = ?`,
			sub.Platform, sub.Email).Scan(&count)
		if count > 0 {
			result.SubsSkipped++
			continue
		}

		_, err := s.db.Exec(`
			INSERT INTO subscriptions (platform, email, category, plan_name, status,
			                          cost_amount, cost_currency, billing_cycle, next_renewal, notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sub.Platform, sub.Email, sub.Category, sub.PlanName, sub.Status,
			sub.CostAmount, sub.CostCurrency, sub.BillingCycle, sub.NextRenewal, sub.Notes)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("subscription insert %q: %v", sub.Platform, err))
			continue
		}
		result.SubsCreated++
	}

	// ── Import snapshots (dedup by account+captured_at within 1s) ──
	for _, raw := range envelope.Snapshots {
		var snap importSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("snapshot parse: %v", err))
			continue
		}

		// Resolve account ID — try email mapping first, then raw account_id
		accountID := snap.AccountID
		if snap.Email != "" {
			if id, ok := emailToID[snap.Email]; ok {
				accountID = id
			}
		}
		if accountID == 0 {
			continue
		}

		// Check for duplicate (same account + same second)
		capturedAt, err := time.Parse(time.RFC3339, snap.CapturedAt)
		if err != nil {
			capturedAt, err = time.Parse("2006-01-02T15:04:05Z", snap.CapturedAt)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("snapshot time parse: %v", err))
				continue
			}
		}
		windowStart := capturedAt.Add(-500 * time.Millisecond).UTC().Format(time.RFC3339)
		windowEnd := capturedAt.Add(500 * time.Millisecond).UTC().Format(time.RFC3339)

		var dupeCount int
		s.db.QueryRow(`
			SELECT COUNT(*) FROM snapshots
			WHERE account_id = ? AND captured_at BETWEEN ? AND ?`,
			accountID, windowStart, windowEnd).Scan(&dupeCount)
		if dupeCount > 0 {
			result.SnapshotsDuped++
			continue
		}

		_, err = s.db.Exec(`
			INSERT INTO snapshots (account_id, captured_at, email, plan_name, models_json,
			                      capture_method, capture_source, source_id)
			VALUES (?, ?, ?, ?, ?, 'imported', 'json', 'import')`,
			accountID, snap.CapturedAt, snap.Email, snap.PlanName, snap.ModelsJSON)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("snapshot insert: %v", err))
			continue
		}
		result.SnapshotsImported++
	}

	return result, nil
}
