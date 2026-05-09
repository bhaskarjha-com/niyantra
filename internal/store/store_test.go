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

	// Verify schema version is 9
	v := s.getUserVersion()
	if v != 9 {
		t.Errorf("expected schema version 9, got %d", v)
	}

	// Insert a snapshot and query it back
	accountID, err := s.GetOrCreateAccount("test@example.com", "Pro")
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

	accountID, _ := s.GetOrCreateAccount("cleanup@example.com", "Free")

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

	accountID, _ := s.GetOrCreateAccount("credits@example.com", "Pro")

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

	accountID, _ := s.GetOrCreateAccount("adjust@example.com", "Pro")

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
