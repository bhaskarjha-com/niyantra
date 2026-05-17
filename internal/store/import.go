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
	ClaudeImported    int      `json:"claudeImported"`
	ClaudeDuped       int      `json:"claudeDuped"`
	CodexImported     int      `json:"codexImported"`
	CodexDuped        int      `json:"codexDuped"`
	CursorImported    int      `json:"cursorImported"`
	CursorDuped       int      `json:"cursorDuped"`
	GeminiImported    int      `json:"geminiImported"`
	GeminiDuped       int      `json:"geminiDuped"`
	CopilotImported   int      `json:"copilotImported"`
	CopilotDuped      int      `json:"copilotDuped"`
	PluginImported    int      `json:"pluginImported"`
	PluginDuped       int      `json:"pluginDuped"`
	Errors            []string `json:"errors"`
}

// importEnvelope is the expected JSON export structure.
type importEnvelope struct {
	Version       string            `json:"version"`
	ExportedAt    string            `json:"exportedAt"`
	Accounts      []json.RawMessage `json:"accounts"`
	Subscriptions []json.RawMessage `json:"subscriptions"`
	Snapshots     []json.RawMessage `json:"snapshots"`
	ClaudeSnaps   []json.RawMessage `json:"claudeSnapshots"`
	CodexSnaps    []json.RawMessage `json:"codexSnapshots"`
	CursorSnaps   []json.RawMessage `json:"cursorSnapshots"`
	GeminiSnaps   []json.RawMessage `json:"geminiSnapshots"`
	CopilotSnaps  []json.RawMessage `json:"copilotSnapshots"`
	PluginSnaps   []json.RawMessage `json:"pluginSnapshots"`
}

type importAccount struct {
	Email    string `json:"email"`
	PlanName string `json:"plan_name"`
	Provider string `json:"provider"`
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

// parseTimeFlexible parses a timestamp string in RFC3339 or ISO8601 format.
func parseTimeFlexible(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006-01-02T15:04:05Z", s)
	if err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}

// dedupWindow returns start/end strings for a ±500ms dedup window around a timestamp.
func dedupWindow(t time.Time) (string, string) {
	start := t.Add(-500 * time.Millisecond).UTC().Format(time.RFC3339)
	end := t.Add(500 * time.Millisecond).UTC().Format(time.RFC3339)
	return start, end
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

		// Default provider to antigravity for legacy imports
		provider := acct.Provider
		if provider == "" {
			provider = "antigravity"
		}

		// Check if exists (dedup by email + provider)
		var existingID int64
		err := s.db.QueryRow(`SELECT id FROM accounts WHERE email = ? AND provider = ?`, acct.Email, provider).Scan(&existingID)
		if err == nil {
			emailToID[acct.Email] = existingID
			result.AccountsSkipped++
			continue
		}

		res, err := s.db.Exec(`INSERT INTO accounts (email, plan_name, provider) VALUES (?, ?, ?)`,
			acct.Email, acct.PlanName, provider)
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
		capturedAt, err := parseTimeFlexible(snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("snapshot time parse: %v", err))
			continue
		}
		windowStart, windowEnd := dedupWindow(capturedAt)

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

	// ── Import Claude snapshots (dedup by captured_at ±500ms) ──
	for _, raw := range envelope.ClaudeSnaps {
		var snap struct {
			FiveHourPct   float64  `json:"fiveHourPct"`
			SevenDayPct   *float64 `json:"sevenDayPct"`
			FiveHourReset *string  `json:"fiveHourReset"`
			SevenDayReset *string  `json:"sevenDayReset"`
			CapturedAt    string   `json:"capturedAt"`
			Source        string   `json:"source"`
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("claude snap parse: %v", err))
			continue
		}
		if snap.CapturedAt == "" {
			continue
		}
		capturedAt, err := parseTimeFlexible(snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("claude snap time: %v", err))
			continue
		}
		wStart, wEnd := dedupWindow(capturedAt)
		var dupeCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM claude_snapshots WHERE captured_at BETWEEN ? AND ?`,
			wStart, wEnd).Scan(&dupeCount)
		if dupeCount > 0 {
			result.ClaudeDuped++
			continue
		}
		source := snap.Source
		if source == "" {
			source = "imported"
		}
		_, err = s.db.Exec(`
			INSERT INTO claude_snapshots (five_hour_pct, seven_day_pct, five_hour_reset, seven_day_reset, captured_at, source)
			VALUES (?, ?, ?, ?, ?, ?)`,
			snap.FiveHourPct, snap.SevenDayPct, snap.FiveHourReset, snap.SevenDayReset,
			snap.CapturedAt, source)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("claude snap insert: %v", err))
			continue
		}
		result.ClaudeImported++
	}

	// ── Import Codex snapshots ──
	for _, raw := range envelope.CodexSnaps {
		var snap struct {
			AccountID     string   `json:"accountId"`
			Email         string   `json:"email"`
			FiveHourPct   float64  `json:"fiveHourPct"`
			SevenDayPct   *float64 `json:"sevenDayPct"`
			PlanType      string   `json:"planType"`
			CreditsBalance *float64 `json:"creditsBalance"`
			CapturedAt    string   `json:"capturedAt"`
			CaptureMethod string   `json:"captureMethod"`
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("codex snap parse: %v", err))
			continue
		}
		if snap.CapturedAt == "" {
			continue
		}
		capturedAt, err := parseTimeFlexible(snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("codex snap time: %v", err))
			continue
		}
		wStart, wEnd := dedupWindow(capturedAt)
		var dupeCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM codex_snapshots WHERE captured_at BETWEEN ? AND ?`,
			wStart, wEnd).Scan(&dupeCount)
		if dupeCount > 0 {
			result.CodexDuped++
			continue
		}
		method := snap.CaptureMethod
		if method == "" {
			method = "imported"
		}
		_, err = s.db.Exec(`
			INSERT INTO codex_snapshots (account_id, email, five_hour_pct, seven_day_pct,
				plan_type, credits_balance, captured_at, capture_method, capture_source)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'json')`,
			snap.AccountID, snap.Email, snap.FiveHourPct, snap.SevenDayPct,
			snap.PlanType, snap.CreditsBalance, snap.CapturedAt, method)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("codex snap insert: %v", err))
			continue
		}
		result.CodexImported++
	}

	// ── Import Cursor snapshots ──
	for _, raw := range envelope.CursorSnaps {
		var snap struct {
			Email       string  `json:"email"`
			PremiumUsed int     `json:"premiumUsed"`
			PremiumLimit int    `json:"premiumLimit"`
			UsagePct    float64 `json:"usagePct"`
			PlanType    string  `json:"planType"`
			CapturedAt  string  `json:"capturedAt"`
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cursor snap parse: %v", err))
			continue
		}
		if snap.CapturedAt == "" {
			continue
		}
		capturedAt, err := parseTimeFlexible(snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cursor snap time: %v", err))
			continue
		}
		wStart, wEnd := dedupWindow(capturedAt)
		var dupeCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM cursor_snapshots WHERE captured_at BETWEEN ? AND ?`,
			wStart, wEnd).Scan(&dupeCount)
		if dupeCount > 0 {
			result.CursorDuped++
			continue
		}
		_, err = s.db.Exec(`
			INSERT INTO cursor_snapshots (email, premium_used, premium_limit, usage_pct,
				plan_type, captured_at, capture_method, capture_source)
			VALUES (?, ?, ?, ?, ?, ?, 'imported', 'json')`,
			snap.Email, snap.PremiumUsed, snap.PremiumLimit, snap.UsagePct,
			snap.PlanType, snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cursor snap insert: %v", err))
			continue
		}
		result.CursorImported++
	}

	// ── Import Gemini snapshots ──
	for _, raw := range envelope.GeminiSnaps {
		var snap struct {
			Email      string  `json:"email"`
			Tier       string  `json:"tier"`
			OverallPct float64 `json:"overallPct"`
			ModelsJSON string  `json:"modelsJson"`
			ProjectID  string  `json:"projectId"`
			CapturedAt string  `json:"capturedAt"`
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("gemini snap parse: %v", err))
			continue
		}
		if snap.CapturedAt == "" {
			continue
		}
		capturedAt, err := parseTimeFlexible(snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("gemini snap time: %v", err))
			continue
		}
		wStart, wEnd := dedupWindow(capturedAt)
		var dupeCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM gemini_snapshots WHERE captured_at BETWEEN ? AND ?`,
			wStart, wEnd).Scan(&dupeCount)
		if dupeCount > 0 {
			result.GeminiDuped++
			continue
		}
		_, err = s.db.Exec(`
			INSERT INTO gemini_snapshots (email, tier, overall_pct, models_json, project_id,
				captured_at, capture_method, capture_source)
			VALUES (?, ?, ?, ?, ?, ?, 'imported', 'json')`,
			snap.Email, snap.Tier, snap.OverallPct, snap.ModelsJSON,
			snap.ProjectID, snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("gemini snap insert: %v", err))
			continue
		}
		result.GeminiImported++
	}

	// ── Import Copilot snapshots ──
	for _, raw := range envelope.CopilotSnaps {
		var snap struct {
			Email      string  `json:"email"`
			Username   string  `json:"username"`
			Plan       string  `json:"plan"`
			PremiumPct float64 `json:"premiumPct"`
			ChatPct    float64 `json:"chatPct"`
			CapturedAt string  `json:"capturedAt"`
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("copilot snap parse: %v", err))
			continue
		}
		if snap.CapturedAt == "" {
			continue
		}
		capturedAt, err := parseTimeFlexible(snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("copilot snap time: %v", err))
			continue
		}
		wStart, wEnd := dedupWindow(capturedAt)
		var dupeCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM copilot_snapshots WHERE captured_at BETWEEN ? AND ?`,
			wStart, wEnd).Scan(&dupeCount)
		if dupeCount > 0 {
			result.CopilotDuped++
			continue
		}
		_, err = s.db.Exec(`
			INSERT INTO copilot_snapshots (email, username, plan, premium_pct, chat_pct,
				captured_at, capture_method, capture_source)
			VALUES (?, ?, ?, ?, ?, ?, 'imported', 'json')`,
			snap.Email, snap.Username, snap.Plan, snap.PremiumPct,
			snap.ChatPct, snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("copilot snap insert: %v", err))
			continue
		}
		result.CopilotImported++
	}

	// ── Import Plugin snapshots (dedup by plugin_id + captured_at ±500ms) ──
	for _, raw := range envelope.PluginSnaps {
		var snap struct {
			PluginID     string  `json:"pluginId"`
			Provider     string  `json:"provider"`
			Label        string  `json:"label"`
			Email        string  `json:"email"`
			UsagePct     float64 `json:"usagePct"`
			UsageDisplay string  `json:"usageDisplay"`
			Plan         string  `json:"plan"`
			ModelsJSON   string  `json:"modelsJson"`
			MetadataJSON string  `json:"metadataJson"`
			CapturedAt   string  `json:"capturedAt"`
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("plugin snap parse: %v", err))
			continue
		}
		if snap.CapturedAt == "" || snap.PluginID == "" {
			continue
		}
		capturedAt, err := parseTimeFlexible(snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("plugin snap time: %v", err))
			continue
		}
		wStart, wEnd := dedupWindow(capturedAt)
		var dupeCount int
		s.db.QueryRow(`SELECT COUNT(*) FROM plugin_snapshots WHERE plugin_id = ? AND captured_at BETWEEN ? AND ?`,
			snap.PluginID, wStart, wEnd).Scan(&dupeCount)
		if dupeCount > 0 {
			result.PluginDuped++
			continue
		}
		_, err = s.db.Exec(`
			INSERT INTO plugin_snapshots (plugin_id, provider, label, email, usage_pct,
				usage_display, plan, models_json, metadata_json, captured_at, capture_method)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'imported')`,
			snap.PluginID, snap.Provider, snap.Label, snap.Email,
			snap.UsagePct, snap.UsageDisplay, snap.Plan,
			snap.ModelsJSON, snap.MetadataJSON, snap.CapturedAt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("plugin snap insert: %v", err))
			continue
		}
		result.PluginImported++
	}

	return result, nil
}
