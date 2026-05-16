package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
)

// openTestDB creates an in-memory Store for testing.
func openTestDB(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAndMigrate(t *testing.T) {
	s := openTestDB(t)

	// Verify schema version is 18
	v := s.getUserVersion()
	if v != 18 {
		t.Errorf("expected schema version 18, got %d", v)
	}

	// Insert a snapshot and query it back
	accountID, err := s.GetOrCreateAccount("test@example.com", "Pro", "antigravity")
	if err != nil {
		t.Fatalf("GetOrCreateAccount: %v", err)
	}

	snap := &client.Snapshot{
		AccountID:     accountID,
		CapturedAt:    time.Now().UTC(),
		Email:         "test@example.com",
		PlanName:      "Pro",
		Models:        []client.ModelQuota{{ModelID: "model-1", RemainingFraction: 0.8, RemainingPercent: 80}},
		CaptureMethod: "manual",
		CaptureSource: "cli",
		SourceID:      "antigravity",
	}

	id, err := s.InsertSnapshot(snap)
	if err != nil {
		t.Fatalf("InsertSnapshot: %v", err)
	}
	if id <= 0 {
		t.Error("expected positive snapshot ID")
	}

	// Query back
	snaps, err := s.LatestPerAccount()
	if err != nil {
		t.Fatalf("LatestPerAccount: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %q", snaps[0].Email)
	}
	if len(snaps[0].Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(snaps[0].Models))
	}
}

func TestBusyTimeoutSet(t *testing.T) {
	s := openTestDB(t)

	var timeout int
	err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&timeout)
	if err != nil {
		t.Fatalf("PRAGMA busy_timeout query failed: %v", err)
	}
	if timeout != 5000 {
		t.Errorf("expected busy_timeout=5000, got %d", timeout)
	}
}

func TestConfigCRUD(t *testing.T) {
	s := openTestDB(t)

	// Test reading seeded config
	val := s.GetConfig("auto_capture")
	if val != "false" {
		t.Errorf("expected auto_capture=false, got %q", val)
	}

	// Test setting config
	old, err := s.SetConfig("auto_capture", "true")
	if err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	if old != "false" {
		t.Errorf("expected old value 'false', got %q", old)
	}

	val = s.GetConfig("auto_capture")
	if val != "true" {
		t.Errorf("expected auto_capture=true, got %q", val)
	}

	// Test bool accessor
	if !s.GetConfigBool("auto_capture") {
		t.Error("expected GetConfigBool to return true")
	}
}

func TestRetentionCleanup(t *testing.T) {
	s := openTestDB(t)

	accountID, _ := s.GetOrCreateAccount("cleanup@example.com", "Free", "antigravity")

	// Insert old snapshot (400 days ago)
	oldTime := time.Now().UTC().Add(-400 * 24 * time.Hour)
	snap := &client.Snapshot{
		AccountID:     accountID,
		CapturedAt:    oldTime,
		Email:         "cleanup@example.com",
		Models:        []client.ModelQuota{{ModelID: "m1", RemainingFraction: 0.5}},
		CaptureMethod: "auto",
		CaptureSource: "server",
		SourceID:      "antigravity",
	}
	_, _ = s.InsertSnapshot(snap)

	// Insert recent snapshot
	snap2 := &client.Snapshot{
		AccountID:     accountID,
		CapturedAt:    time.Now().UTC(),
		Email:         "cleanup@example.com",
		Models:        []client.ModelQuota{{ModelID: "m1", RemainingFraction: 0.9}},
		CaptureMethod: "manual",
		CaptureSource: "ui",
		SourceID:      "antigravity",
	}
	_, _ = s.InsertSnapshot(snap2)

	deleted, err := s.DeleteSnapshotsOlderThan(365)
	if err != nil {
		t.Fatalf("DeleteSnapshotsOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	count := s.SnapshotCount()
	if count != 1 {
		t.Errorf("expected 1 remaining snapshot, got %d", count)
	}
}

func TestInsertAndQuerySnapshotWithAICredits(t *testing.T) {
	s := openTestDB(t)

	accountID, _ := s.GetOrCreateAccount("credits@example.com", "Pro", "antigravity")

	snap := &client.Snapshot{
		AccountID:     accountID,
		CapturedAt:    time.Now().UTC(),
		Email:         "credits@example.com",
		PlanName:      "Pro",
		Models:        []client.ModelQuota{{ModelID: "m1", RemainingFraction: 1.0, RemainingPercent: 100}},
		AICredits:     []client.AICredit{{CreditType: "GOOGLE_ONE_AI", CreditAmount: 1000, MinimumForUsage: 0}},
		CaptureMethod: "manual",
		CaptureSource: "cli",
		SourceID:      "antigravity",
	}

	_, err := s.InsertSnapshot(snap)
	if err != nil {
		t.Fatalf("InsertSnapshot with AI credits: %v", err)
	}

	snaps, _ := s.LatestPerAccount()
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if len(snaps[0].AICredits) != 1 {
		t.Errorf("expected 1 AI credit, got %d", len(snaps[0].AICredits))
	}

	// Verify JSON roundtrip
	b, _ := json.Marshal(snaps[0].AICredits)
	var credits []client.AICredit
	json.Unmarshal(b, &credits)
	if credits[0].CreditAmount != 1000 {
		t.Errorf("expected credit amount 1000, got %f", credits[0].CreditAmount)
	}
}

func TestUpdateSnapshotModels(t *testing.T) {
	s := openTestDB(t)

	accountID, _ := s.GetOrCreateAccount("adjust@example.com", "Pro", "antigravity")

	snap := &client.Snapshot{
		AccountID:  accountID,
		CapturedAt: time.Now().UTC(),
		Email:      "adjust@example.com",
		PlanName:   "Pro",
		Models: []client.ModelQuota{
			{ModelID: "claude-sonnet", Label: "Claude Sonnet 4.6", RemainingFraction: 0.8, RemainingPercent: 80},
			{ModelID: "gemini-pro", Label: "Gemini 3.1 Pro", RemainingFraction: 1.0, RemainingPercent: 100},
		},
		CaptureMethod: "manual",
		CaptureSource: "ui",
		SourceID:      "antigravity",
	}

	snapID, err := s.InsertSnapshot(snap)
	if err != nil {
		t.Fatalf("InsertSnapshot: %v", err)
	}

	// Adjust Claude Sonnet from 80% to 45%
	updatedModels := []client.ModelQuota{
		{ModelID: "claude-sonnet", Label: "Claude Sonnet 4.6", RemainingFraction: 0.45, RemainingPercent: 45},
		{ModelID: "gemini-pro", Label: "Gemini 3.1 Pro", RemainingFraction: 1.0, RemainingPercent: 100},
	}

	if err := s.UpdateSnapshotModels(snapID, updatedModels); err != nil {
		t.Fatalf("UpdateSnapshotModels: %v", err)
	}

	// Verify the update persisted
	snaps, err := s.LatestPerAccount()
	if err != nil {
		t.Fatalf("LatestPerAccount: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if len(snaps[0].Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(snaps[0].Models))
	}

	// Check the adjusted model
	for _, m := range snaps[0].Models {
		if m.Label == "Claude Sonnet 4.6" {
			if m.RemainingPercent != 45 {
				t.Errorf("expected Claude Sonnet adjusted to 45%%, got %.0f%%", m.RemainingPercent)
			}
			if m.RemainingFraction != 0.45 {
				t.Errorf("expected fraction 0.45, got %f", m.RemainingFraction)
			}
		}
		if m.Label == "Gemini 3.1 Pro" {
			if m.RemainingPercent != 100 {
				t.Errorf("expected Gemini Pro unchanged at 100%%, got %.0f%%", m.RemainingPercent)
			}
		}
	}
}

func TestUpdateSnapshotModels_NotFound(t *testing.T) {
	s := openTestDB(t)

	// Try to update a non-existent snapshot
	err := s.UpdateSnapshotModels(99999, []client.ModelQuota{})
	if err == nil {
		t.Error("expected error for non-existent snapshot, got nil")
	}
}

func TestAccountMeta(t *testing.T) {
	s := openTestDB(t)

	accountID, err := s.GetOrCreateAccount("meta@example.com", "Pro", "antigravity")
	if err != nil {
		t.Fatalf("GetOrCreateAccount: %v", err)
	}

	// Default meta should be empty/zero
	notes, tags, pinned, renewalDay, err := s.AccountMeta(accountID)
	if err != nil {
		t.Fatalf("AccountMeta: %v", err)
	}
	if notes != "" || tags != "" || pinned != "" || renewalDay != 0 {
		t.Errorf("expected empty defaults, got notes=%q tags=%q pinned=%q renewalDay=%d", notes, tags, pinned, renewalDay)
	}

	// Update meta
	if err := s.UpdateAccountMeta(accountID, "Test account", "work,primary", "claude_gpt", 7); err != nil {
		t.Fatalf("UpdateAccountMeta: %v", err)
	}

	// Verify read-back
	notes, tags, pinned, renewalDay, err = s.AccountMeta(accountID)
	if err != nil {
		t.Fatalf("AccountMeta after update: %v", err)
	}
	if notes != "Test account" {
		t.Errorf("expected notes 'Test account', got %q", notes)
	}
	if tags != "work,primary" {
		t.Errorf("expected tags 'work,primary', got %q", tags)
	}
	if pinned != "claude_gpt" {
		t.Errorf("expected pinned 'claude_gpt', got %q", pinned)
	}
	if renewalDay != 7 {
		t.Errorf("expected renewalDay 7, got %d", renewalDay)
	}

	// Verify AllAccounts includes meta
	accounts, err := s.AllAccounts()
	if err != nil {
		t.Fatalf("AllAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Notes != "Test account" {
		t.Errorf("AllAccounts.Notes: expected 'Test account', got %q", accounts[0].Notes)
	}
	if accounts[0].Tags != "work,primary" {
		t.Errorf("AllAccounts.Tags: expected 'work,primary', got %q", accounts[0].Tags)
	}
	if accounts[0].PinnedGroup != "claude_gpt" {
		t.Errorf("AllAccounts.PinnedGroup: expected 'claude_gpt', got %q", accounts[0].PinnedGroup)
	}
	if accounts[0].CreditRenewalDay != 7 {
		t.Errorf("AllAccounts.CreditRenewalDay: expected 7, got %d", accounts[0].CreditRenewalDay)
	}

	// Partial update — only notes
	if err := s.UpdateAccountMeta(accountID, "Updated note", "work,primary", "claude_gpt", 7); err != nil {
		t.Fatalf("UpdateAccountMeta partial: %v", err)
	}
	notes, _, _, _, _ = s.AccountMeta(accountID)
	if notes != "Updated note" {
		t.Errorf("expected notes 'Updated note', got %q", notes)
	}

	// Clear all meta
	if err := s.UpdateAccountMeta(accountID, "", "", "", 0); err != nil {
		t.Fatalf("UpdateAccountMeta clear: %v", err)
	}
	notes, tags, pinned, renewalDay, _ = s.AccountMeta(accountID)
	if notes != "" || tags != "" || pinned != "" || renewalDay != 0 {
		t.Errorf("expected empty after clear, got notes=%q tags=%q pinned=%q renewalDay=%d", notes, tags, pinned, renewalDay)
	}
}

func TestPinnedGroupPartialUpdate(t *testing.T) {
	s := openTestDB(t)

	accountID, err := s.GetOrCreateAccount("pin@example.com", "Pro", "antigravity")
	if err != nil {
		t.Fatalf("GetOrCreateAccount: %v", err)
	}

	// Set initial meta: notes + tags + no pinned group
	if err := s.UpdateAccountMeta(accountID, "My notes", "work,dev", "", 0); err != nil {
		t.Fatalf("UpdateAccountMeta initial: %v", err)
	}

	// Pin a group — should preserve notes and tags
	if err := s.UpdateAccountMeta(accountID, "My notes", "work,dev", "gemini_pro", 0); err != nil {
		t.Fatalf("UpdateAccountMeta pin: %v", err)
	}

	notes, tags, pinned, _, err := s.AccountMeta(accountID)
	if err != nil {
		t.Fatalf("AccountMeta after pin: %v", err)
	}
	if notes != "My notes" {
		t.Errorf("pinning should preserve notes, got %q", notes)
	}
	if tags != "work,dev" {
		t.Errorf("pinning should preserve tags, got %q", tags)
	}
	if pinned != "gemini_pro" {
		t.Errorf("expected pinned 'gemini_pro', got %q", pinned)
	}

	// Change pin to another group
	if err := s.UpdateAccountMeta(accountID, "My notes", "work,dev", "gemini_flash", 0); err != nil {
		t.Fatalf("UpdateAccountMeta repin: %v", err)
	}

	_, _, pinned, _, _ = s.AccountMeta(accountID)
	if pinned != "gemini_flash" {
		t.Errorf("expected pinned 'gemini_flash', got %q", pinned)
	}

	// Unpin (clear pinned group)
	if err := s.UpdateAccountMeta(accountID, "My notes", "work,dev", "", 0); err != nil {
		t.Fatalf("UpdateAccountMeta unpin: %v", err)
	}

	notes, tags, pinned, _, _ = s.AccountMeta(accountID)
	if pinned != "" {
		t.Errorf("expected empty pinned after unpin, got %q", pinned)
	}
	if notes != "My notes" || tags != "work,dev" {
		t.Errorf("unpin should preserve notes+tags, got notes=%q tags=%q", notes, tags)
	}
}

func TestCreditRenewalDay(t *testing.T) {
	s := openTestDB(t)

	accountID, err := s.GetOrCreateAccount("renewal@example.com", "Pro", "antigravity")
	if err != nil {
		t.Fatalf("GetOrCreateAccount: %v", err)
	}

	// Default renewal day is 0 (not set)
	_, _, _, renewalDay, err := s.AccountMeta(accountID)
	if err != nil {
		t.Fatalf("AccountMeta: %v", err)
	}
	if renewalDay != 0 {
		t.Errorf("expected default renewalDay=0, got %d", renewalDay)
	}

	// Set renewal day to 7 (from Google One AI credits page)
	if err := s.UpdateAccountMeta(accountID, "Main account", "work", "", 7); err != nil {
		t.Fatalf("UpdateAccountMeta set renewalDay: %v", err)
	}

	notes, tags, _, renewalDay, _ := s.AccountMeta(accountID)
	if renewalDay != 7 {
		t.Errorf("expected renewalDay=7, got %d", renewalDay)
	}
	if notes != "Main account" || tags != "work" {
		t.Errorf("setting renewalDay should preserve other fields, got notes=%q tags=%q", notes, tags)
	}

	// Change renewal day (e.g., plan changed)
	if err := s.UpdateAccountMeta(accountID, "Main account", "work", "", 15); err != nil {
		t.Fatalf("UpdateAccountMeta change renewalDay: %v", err)
	}

	_, _, _, renewalDay, _ = s.AccountMeta(accountID)
	if renewalDay != 15 {
		t.Errorf("expected renewalDay=15, got %d", renewalDay)
	}

	// Clear renewal day
	if err := s.UpdateAccountMeta(accountID, "Main account", "work", "", 0); err != nil {
		t.Fatalf("UpdateAccountMeta clear renewalDay: %v", err)
	}

	_, _, _, renewalDay, _ = s.AccountMeta(accountID)
	if renewalDay != 0 {
		t.Errorf("expected renewalDay=0 after clear, got %d", renewalDay)
	}

	// Verify AllAccounts surfaces creditRenewalDay
	if err := s.UpdateAccountMeta(accountID, "", "", "", 22); err != nil {
		t.Fatalf("UpdateAccountMeta for AllAccounts check: %v", err)
	}
	accounts, err := s.AllAccounts()
	if err != nil {
		t.Fatalf("AllAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].CreditRenewalDay != 22 {
		t.Errorf("AllAccounts.CreditRenewalDay: expected 22, got %d", accounts[0].CreditRenewalDay)
	}
}

func TestModelPricingDefaults(t *testing.T) {
	s := openTestDB(t)

	// First call should seed defaults
	prices, err := s.GetModelPricing()
	if err != nil {
		t.Fatalf("GetModelPricing: %v", err)
	}
	if len(prices) != 6 {
		t.Fatalf("expected 6 default models, got %d", len(prices))
	}

	// Verify specific defaults
	var found bool
	for _, p := range prices {
		if p.ModelID == "claude-sonnet-4.6" {
			found = true
			if p.InputPer1M != 3.00 {
				t.Errorf("Claude Sonnet input: expected 3.00, got %f", p.InputPer1M)
			}
			if p.OutputPer1M != 15.00 {
				t.Errorf("Claude Sonnet output: expected 15.00, got %f", p.OutputPer1M)
			}
			if p.CachePer1M != 0.30 {
				t.Errorf("Claude Sonnet cache: expected 0.30, got %f", p.CachePer1M)
			}
			if p.Provider != "anthropic" {
				t.Errorf("Claude Sonnet provider: expected 'anthropic', got %q", p.Provider)
			}
		}
	}
	if !found {
		t.Error("claude-sonnet-4.6 not found in defaults")
	}

	// Verify config key was created
	raw := s.GetConfig("model_pricing")
	if raw == "" {
		t.Error("expected model_pricing config key to be set after first access")
	}

	// Second call should return same data (from stored config, not re-seed)
	prices2, err := s.GetModelPricing()
	if err != nil {
		t.Fatalf("GetModelPricing second call: %v", err)
	}
	if len(prices2) != 6 {
		t.Fatalf("expected 6 models on second call, got %d", len(prices2))
	}
}

func TestModelPricingCustomUpdate(t *testing.T) {
	s := openTestDB(t)

	// Seed defaults
	_, _ = s.GetModelPricing()

	// Update with custom pricing
	custom := []ModelPrice{
		{ModelID: "claude-sonnet-4.6", DisplayName: "Claude Sonnet 4.6", Provider: "anthropic", InputPer1M: 4.00, OutputPer1M: 20.00, CachePer1M: 0.40},
		{ModelID: "my-custom-model", DisplayName: "My Custom Model", Provider: "custom", InputPer1M: 10.00, OutputPer1M: 30.00, CachePer1M: 2.00},
	}

	if err := s.SetModelPricing(custom); err != nil {
		t.Fatalf("SetModelPricing: %v", err)
	}

	// Read back
	prices, err := s.GetModelPricing()
	if err != nil {
		t.Fatalf("GetModelPricing after update: %v", err)
	}
	if len(prices) != 2 {
		t.Fatalf("expected 2 custom models, got %d", len(prices))
	}

	// Verify updated values
	if prices[0].InputPer1M != 4.00 {
		t.Errorf("expected custom input 4.00, got %f", prices[0].InputPer1M)
	}
	if prices[1].ModelID != "my-custom-model" {
		t.Errorf("expected custom model ID, got %q", prices[1].ModelID)
	}
}

func TestGetModelPrice(t *testing.T) {
	s := openTestDB(t)

	// Seed defaults
	_, _ = s.GetModelPricing()

	// Lookup existing model
	p := s.GetModelPrice("gpt-4o")
	if p == nil {
		t.Fatal("expected to find gpt-4o pricing")
	}
	if p.InputPer1M != 2.50 {
		t.Errorf("GPT-4o input: expected 2.50, got %f", p.InputPer1M)
	}
	if p.OutputPer1M != 10.00 {
		t.Errorf("GPT-4o output: expected 10.00, got %f", p.OutputPer1M)
	}

	// Lookup non-existent model
	p = s.GetModelPrice("nonexistent-model")
	if p != nil {
		t.Error("expected nil for nonexistent model")
	}

	// Fuzzy prefix match: Claude Code JSONL uses "claude-sonnet-4" but
	// pricing config has "claude-sonnet-4.6" — should match via prefix
	p = s.GetModelPrice("claude-sonnet-4")
	if p == nil {
		t.Fatal("expected fuzzy match for claude-sonnet-4 → claude-sonnet-4.6")
	}
	if p.ModelID != "claude-sonnet-4.6" {
		t.Errorf("expected match to claude-sonnet-4.6, got %s", p.ModelID)
	}
	if p.InputPer1M != 3.00 {
		t.Errorf("expected input 3.00 for sonnet, got %f", p.InputPer1M)
	}

	// Fuzzy prefix match: opus variant
	p = s.GetModelPrice("claude-opus-4")
	if p == nil {
		t.Fatal("expected fuzzy match for claude-opus-4 → claude-opus-4.6")
	}
	if p.ModelID != "claude-opus-4.6" {
		t.Errorf("expected match to claude-opus-4.6, got %s", p.ModelID)
	}
}

func TestHeatmapData(t *testing.T) {
	s := openTestDB(t)

	// Empty heatmap should return no error
	days, err := s.HeatmapData(365)
	if err != nil {
		t.Fatalf("HeatmapData empty: %v", err)
	}
	if len(days) != 0 {
		t.Errorf("expected 0 days, got %d", len(days))
	}

	// Insert snapshots across all 3 providers
	accountID, _ := s.GetOrCreateAccount("heatmap@example.com", "Pro", "antigravity")

	// Antigravity snapshot (today)
	snap := &client.Snapshot{
		AccountID:     accountID,
		CapturedAt:    time.Now().UTC(),
		Email:         "heatmap@example.com",
		PlanName:      "Pro",
		Models:        []client.ModelQuota{{ModelID: "m1", RemainingFraction: 0.5}},
		CaptureMethod: "auto",
		CaptureSource: "server",
		SourceID:      "antigravity",
	}
	_, _ = s.InsertSnapshot(snap)
	_, _ = s.InsertSnapshot(snap) // 2 AG snaps today

	// Claude snapshot (today)
	_, _ = s.InsertClaudeSnapshot(50.0, nil, nil, nil, "statusline", nil)

	// Codex snapshot (today)
	codexSnap := &CodexSnapshot{
		FiveHourPct:   30.0,
		CaptureMethod: "auto",
		CaptureSource: "api",
	}
	_, _ = s.InsertCodexSnapshot(codexSnap)

	// Cursor snapshot (today)
	cursorSnap := &CursorSnapshot{
		RequestsUsed:  50,
		RequestsMax:   500,
		UsagePct:      10.0,
		BillingModel:  "request_count",
		CaptureMethod: "auto",
		CaptureSource: "server",
	}
	_, _ = s.InsertCursorSnapshot(cursorSnap)

	// Gemini snapshot (today)
	geminiSnap := &GeminiSnapshot{
		Tier:          "standard",
		OverallPct:    25.0,
		ModelsJSON:    `[{"modelId":"gemini-2.5-flash","remainingFraction":0.75}]`,
		CaptureMethod: "auto",
		CaptureSource: "server",
	}
	_, _ = s.InsertGeminiSnapshot(geminiSnap)

	// Copilot snapshot (today)
	copilotSnap := &CopilotSnapshot{
		Plan:          "Pro",
		PremiumPct:    25.0,
		HasPremium:    true,
		CaptureMethod: "auto",
		CaptureSource: "server",
	}
	_, _ = s.InsertCopilotSnapshot(copilotSnap)

	// Query heatmap
	days, err = s.HeatmapData(365)
	if err != nil {
		t.Fatalf("HeatmapData with data: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}

	day := days[0]
	if day.Antigravity != 2 {
		t.Errorf("expected 2 AG snapshots, got %d", day.Antigravity)
	}
	if day.Claude != 1 {
		t.Errorf("expected 1 Claude snapshot, got %d", day.Claude)
	}
	if day.Codex != 1 {
		t.Errorf("expected 1 Codex snapshot, got %d", day.Codex)
	}
	if day.Cursor != 1 {
		t.Errorf("expected 1 Cursor snapshot, got %d", day.Cursor)
	}
	if day.Gemini != 1 {
		t.Errorf("expected 1 Gemini snapshot, got %d", day.Gemini)
	}
	if day.Copilot != 1 {
		t.Errorf("expected 1 Copilot snapshot, got %d", day.Copilot)
	}
	if day.Count != 7 {
		t.Errorf("expected total count 7, got %d", day.Count)
	}
}

func TestUnifiedAccountModel(t *testing.T) {
	s := openTestDB(t)

	// Same email, different providers — should create separate accounts
	agID, err := s.GetOrCreateAccount("user@example.com", "Pro", "antigravity")
	if err != nil {
		t.Fatalf("GetOrCreateAccount antigravity: %v", err)
	}

	codexID, err := s.GetOrCreateAccount("user@example.com", "Plus", "codex")
	if err != nil {
		t.Fatalf("GetOrCreateAccount codex: %v", err)
	}

	cursorID, err := s.GetOrCreateAccount("user@example.com", "Pro", "cursor")
	if err != nil {
		t.Fatalf("GetOrCreateAccount cursor: %v", err)
	}

	// All three IDs must be different
	if agID == codexID || agID == cursorID || codexID == cursorID {
		t.Errorf("expected different account IDs, got ag=%d codex=%d cursor=%d", agID, codexID, cursorID)
	}

	// Should have 3 accounts total
	count := s.AccountCount()
	if count != 3 {
		t.Errorf("expected 3 accounts, got %d", count)
	}

	// Metadata should be independent per provider
	if err := s.UpdateAccountMeta(agID, "AG note", "work", "", 0); err != nil {
		t.Fatalf("UpdateAccountMeta ag: %v", err)
	}
	if err := s.UpdateAccountMeta(codexID, "Codex note", "dev", "", 0); err != nil {
		t.Fatalf("UpdateAccountMeta codex: %v", err)
	}

	agNotes, agTags, _, _, _ := s.AccountMeta(agID)
	cxNotes, cxTags, _, _, _ := s.AccountMeta(codexID)

	if agNotes != "AG note" || agTags != "work" {
		t.Errorf("AG meta wrong: notes=%q tags=%q", agNotes, agTags)
	}
	if cxNotes != "Codex note" || cxTags != "dev" {
		t.Errorf("Codex meta wrong: notes=%q tags=%q", cxNotes, cxTags)
	}

	// Upsert same provider should NOT create new account
	agID2, _ := s.GetOrCreateAccount("user@example.com", "Ultra", "antigravity")
	if agID2 != agID {
		t.Errorf("upsert should return same ID: got %d, want %d", agID2, agID)
	}

	// AllAccounts should include provider field
	accounts, err := s.AllAccounts()
	if err != nil {
		t.Fatalf("AllAccounts: %v", err)
	}
	if len(accounts) != 3 {
		t.Fatalf("expected 3 accounts in AllAccounts, got %d", len(accounts))
	}

	providers := map[string]bool{}
	for _, a := range accounts {
		providers[a.Provider] = true
	}
	if !providers["antigravity"] || !providers["codex"] || !providers["cursor"] {
		t.Errorf("expected all 3 providers, got %v", providers)
	}
}

func TestTokenUsageCRUD(t *testing.T) {
	s := openTestDB(t)

	// Insert a token usage row
	row := &TokenUsageRow{
		Date:          "2026-05-14",
		Provider:      "claude",
		Model:         "claude-sonnet-4.6",
		InputTokens:   100000,
		OutputTokens:  50000,
		CacheRead:     20000,
		CacheCreate:   5000,
		EstimatedCost: 1.50,
		TurnCount:     42,
		SessionCount:  3,
		Source:        "parsed",
	}

	if err := s.InsertTokenUsage(row); err != nil {
		t.Fatalf("InsertTokenUsage: %v", err)
	}

	// Query back
	rows, err := s.QueryTokenUsage("2026-05-01", "claude")
	if err != nil {
		t.Fatalf("QueryTokenUsage: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].InputTokens != 100000 {
		t.Errorf("expected input tokens 100000, got %d", rows[0].InputTokens)
	}
	if rows[0].OutputTokens != 50000 {
		t.Errorf("expected output tokens 50000, got %d", rows[0].OutputTokens)
	}
	if rows[0].EstimatedCost != 1.50 {
		t.Errorf("expected cost 1.50, got %f", rows[0].EstimatedCost)
	}
	if rows[0].Source != "parsed" {
		t.Errorf("expected source 'parsed', got %q", rows[0].Source)
	}
}

func TestTokenUsageUpsert(t *testing.T) {
	s := openTestDB(t)

	// Insert initial row
	row := &TokenUsageRow{
		Date:          "2026-05-14",
		Provider:      "claude",
		Model:         "claude-sonnet-4.6",
		InputTokens:   100000,
		OutputTokens:  50000,
		EstimatedCost: 1.50,
		TurnCount:     42,
		Source:        "parsed",
	}
	if err := s.InsertTokenUsage(row); err != nil {
		t.Fatalf("InsertTokenUsage first: %v", err)
	}

	// Upsert with updated values (same date+provider+model key)
	row.InputTokens = 200000
	row.OutputTokens = 100000
	row.EstimatedCost = 3.00
	row.TurnCount = 84
	if err := s.InsertTokenUsage(row); err != nil {
		t.Fatalf("InsertTokenUsage upsert: %v", err)
	}

	// Should still be 1 row, with updated values
	rows, err := s.QueryTokenUsage("2026-05-01", "claude")
	if err != nil {
		t.Fatalf("QueryTokenUsage: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after upsert, got %d", len(rows))
	}
	if rows[0].InputTokens != 200000 {
		t.Errorf("expected input 200000 after upsert, got %d", rows[0].InputTokens)
	}
	if rows[0].EstimatedCost != 3.00 {
		t.Errorf("expected cost 3.00 after upsert, got %f", rows[0].EstimatedCost)
	}
}

func TestTokenUsageProviderFilter(t *testing.T) {
	s := openTestDB(t)

	// Insert rows for multiple providers
	providers := []string{"claude", "antigravity", "codex"}
	for _, p := range providers {
		if err := s.InsertTokenUsage(&TokenUsageRow{
			Date:          "2026-05-14",
			Provider:      p,
			Model:         p + "-model",
			InputTokens:   10000,
			OutputTokens:  5000,
			EstimatedCost: 0.50,
			Source:        "parsed",
		}); err != nil {
			t.Fatalf("InsertTokenUsage(%s): %v", p, err)
		}
	}

	// Query all providers
	allRows, err := s.QueryTokenUsage("2026-05-01", "all")
	if err != nil {
		t.Fatalf("QueryTokenUsage(all): %v", err)
	}
	if len(allRows) != 3 {
		t.Fatalf("expected 3 rows for 'all', got %d", len(allRows))
	}

	// Query specific provider
	claudeRows, err := s.QueryTokenUsage("2026-05-01", "claude")
	if err != nil {
		t.Fatalf("QueryTokenUsage(claude): %v", err)
	}
	if len(claudeRows) != 1 {
		t.Fatalf("expected 1 row for 'claude', got %d", len(claudeRows))
	}
	if claudeRows[0].Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", claudeRows[0].Provider)
	}

	// Query empty provider (should return all)
	emptyRows, err := s.QueryTokenUsage("2026-05-01", "")
	if err != nil {
		t.Fatalf("QueryTokenUsage(''): %v", err)
	}
	if len(emptyRows) != 3 {
		t.Fatalf("expected 3 rows for empty provider, got %d", len(emptyRows))
	}
}

func TestTokenUsageDateFilter(t *testing.T) {
	s := openTestDB(t)

	// Insert rows across multiple dates
	dates := []string{"2026-05-01", "2026-05-07", "2026-05-14"}
	for _, d := range dates {
		if err := s.InsertTokenUsage(&TokenUsageRow{
			Date:          d,
			Provider:      "claude",
			Model:         "model-a",
			InputTokens:   10000,
			EstimatedCost: 0.50,
			Source:        "parsed",
		}); err != nil {
			t.Fatalf("InsertTokenUsage(%s): %v", d, err)
		}
	}

	// Filter: only rows after May 5
	rows, err := s.QueryTokenUsage("2026-05-05", "all")
	if err != nil {
		t.Fatalf("QueryTokenUsage: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows after May 5, got %d", len(rows))
	}
}

func TestTokenUsageRetention(t *testing.T) {
	s := openTestDB(t)

	// Insert old and recent rows
	if err := s.InsertTokenUsage(&TokenUsageRow{
		Date: "2025-01-01", Provider: "claude", Model: "old-model",
		InputTokens: 1000, Source: "parsed",
	}); err != nil {
		t.Fatalf("InsertTokenUsage old: %v", err)
	}
	if err := s.InsertTokenUsage(&TokenUsageRow{
		Date: "2026-05-14", Provider: "claude", Model: "new-model",
		InputTokens: 2000, Source: "parsed",
	}); err != nil {
		t.Fatalf("InsertTokenUsage new: %v", err)
	}

	// Delete rows before 2026-01-01
	deleted, err := s.DeleteTokenUsageBefore("2026-01-01")
	if err != nil {
		t.Fatalf("DeleteTokenUsageBefore: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Should have 1 remaining row
	rows, err := s.QueryTokenUsage("2025-01-01", "all")
	if err != nil {
		t.Fatalf("QueryTokenUsage after delete: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after delete, got %d", len(rows))
	}
	if rows[0].Date != "2026-05-14" {
		t.Errorf("expected remaining row date '2026-05-14', got %q", rows[0].Date)
	}
}
