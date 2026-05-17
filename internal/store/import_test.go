package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testImportStore creates a fresh store for import testing.
func testImportStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "import_test.db"))
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// buildExportJSON creates a complete export envelope for testing.
func buildExportJSON(t *testing.T, data map[string]interface{}) []byte {
	t.Helper()
	if _, ok := data["version"]; !ok {
		data["version"] = "1.0"
	}
	if _, ok := data["exportedAt"]; !ok {
		data["exportedAt"] = time.Now().UTC().Format(time.RFC3339)
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}
	return b
}

func TestImportJSON_EmptyEnvelope(t *testing.T) {
	s := testImportStore(t)
	result, err := s.ImportJSON([]byte(`{"version":"1.0"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AccountsCreated != 0 || result.SnapshotsImported != 0 {
		t.Errorf("expected zero imports for empty envelope")
	}
}

func TestImportJSON_InvalidJSON(t *testing.T) {
	s := testImportStore(t)
	_, err := s.ImportJSON([]byte(`{not json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestImportJSON_UnsupportedVersion(t *testing.T) {
	s := testImportStore(t)
	_, err := s.ImportJSON([]byte(`{"version":"99.0"}`))
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestImportJSON_LegacyVersion(t *testing.T) {
	s := testImportStore(t)
	result, err := s.ImportJSON([]byte(`{"version":"niyantra-export-v1"}`))
	if err != nil {
		t.Fatalf("legacy version should be accepted: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestImportJSON_AccountsDedup(t *testing.T) {
	s := testImportStore(t)
	data := buildExportJSON(t, map[string]interface{}{
		"accounts": []map[string]string{
			{"email": "test@example.com", "plan_name": "Pro", "provider": "antigravity"},
		},
	})
	result1, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("first import failed: %v", err)
	}
	if result1.AccountsCreated != 1 {
		t.Errorf("expected 1 account created, got %d", result1.AccountsCreated)
	}

	// Second import should skip
	result2, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}
	if result2.AccountsSkipped != 1 {
		t.Errorf("expected 1 account skipped on re-import, got %d", result2.AccountsSkipped)
	}
	if result2.AccountsCreated != 0 {
		t.Errorf("expected 0 accounts created on re-import, got %d", result2.AccountsCreated)
	}
}

func TestImportJSON_Subscriptions(t *testing.T) {
	s := testImportStore(t)
	data := buildExportJSON(t, map[string]interface{}{
		"subscriptions": []map[string]interface{}{
			{"platform": "GitHub", "email": "test@example.com", "plan_name": "Pro", "status": "active", "cost_amount": 19.99, "cost_currency": "USD", "billing_cycle": "monthly"},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.SubsCreated != 1 {
		t.Errorf("expected 1 subscription created, got %d", result.SubsCreated)
	}

	// Re-import should dedup
	result2, _ := s.ImportJSON(data)
	if result2.SubsSkipped != 1 {
		t.Errorf("expected 1 subscription skipped, got %d", result2.SubsSkipped)
	}
}

func TestImportJSON_ClaudeSnapshots(t *testing.T) {
	s := testImportStore(t)
	capturedAt := time.Now().UTC().Format(time.RFC3339)
	data := buildExportJSON(t, map[string]interface{}{
		"claudeSnapshots": []map[string]interface{}{
			{"fiveHourPct": 45.0, "capturedAt": capturedAt, "source": "statusline"},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.ClaudeImported != 1 {
		t.Errorf("expected 1 claude snapshot imported, got %d", result.ClaudeImported)
	}

	// Dedup check
	result2, _ := s.ImportJSON(data)
	if result2.ClaudeDuped != 1 {
		t.Errorf("expected 1 claude snapshot duped, got %d", result2.ClaudeDuped)
	}
}

func TestImportJSON_CodexSnapshots(t *testing.T) {
	s := testImportStore(t)
	capturedAt := time.Now().UTC().Format(time.RFC3339)
	data := buildExportJSON(t, map[string]interface{}{
		"codexSnapshots": []map[string]interface{}{
			{"accountId": "org-test123", "email": "dev@openai.com", "fiveHourPct": 30.0, "planType": "pro", "capturedAt": capturedAt},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.CodexImported != 1 {
		t.Errorf("expected 1 codex snapshot imported, got %d", result.CodexImported)
	}

	// Dedup check
	result2, _ := s.ImportJSON(data)
	if result2.CodexDuped != 1 {
		t.Errorf("expected 1 codex snapshot duped, got %d", result2.CodexDuped)
	}
}

func TestImportJSON_CursorSnapshots(t *testing.T) {
	s := testImportStore(t)
	capturedAt := time.Now().UTC().Format(time.RFC3339)
	data := buildExportJSON(t, map[string]interface{}{
		"cursorSnapshots": []map[string]interface{}{
			{"email": "user@cursor.sh", "premiumUsed": 100, "premiumLimit": 500, "usagePct": 20.0, "planType": "pro", "capturedAt": capturedAt},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.CursorImported != 1 {
		t.Errorf("expected 1 cursor snapshot imported, got %d", result.CursorImported)
	}

	result2, _ := s.ImportJSON(data)
	if result2.CursorDuped != 1 {
		t.Errorf("expected 1 cursor snapshot duped, got %d", result2.CursorDuped)
	}
}

func TestImportJSON_GeminiSnapshots(t *testing.T) {
	s := testImportStore(t)
	capturedAt := time.Now().UTC().Format(time.RFC3339)
	data := buildExportJSON(t, map[string]interface{}{
		"geminiSnapshots": []map[string]interface{}{
			{"email": "user@gmail.com", "tier": "standard", "overallPct": 25.0, "projectId": "proj-1", "capturedAt": capturedAt},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.GeminiImported != 1 {
		t.Errorf("expected 1 gemini snapshot imported, got %d", result.GeminiImported)
	}

	result2, _ := s.ImportJSON(data)
	if result2.GeminiDuped != 1 {
		t.Errorf("expected 1 gemini snapshot duped, got %d", result2.GeminiDuped)
	}
}

func TestImportJSON_CopilotSnapshots(t *testing.T) {
	s := testImportStore(t)
	capturedAt := time.Now().UTC().Format(time.RFC3339)
	data := buildExportJSON(t, map[string]interface{}{
		"copilotSnapshots": []map[string]interface{}{
			{"email": "dev@github.com", "username": "octocat", "plan": "Pro+", "premiumPct": 50.0, "chatPct": 30.0, "capturedAt": capturedAt},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.CopilotImported != 1 {
		t.Errorf("expected 1 copilot snapshot imported, got %d", result.CopilotImported)
	}

	result2, _ := s.ImportJSON(data)
	if result2.CopilotDuped != 1 {
		t.Errorf("expected 1 copilot snapshot duped, got %d", result2.CopilotDuped)
	}
}

func TestImportJSON_PluginSnapshots(t *testing.T) {
	s := testImportStore(t)
	capturedAt := time.Now().UTC().Format(time.RFC3339)
	data := buildExportJSON(t, map[string]interface{}{
		"pluginSnapshots": []map[string]interface{}{
			{"pluginId": "vercel-quota", "provider": "vercel", "label": "Vercel", "email": "user@vercel.com", "usagePct": 60.0, "plan": "Pro", "capturedAt": capturedAt},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.PluginImported != 1 {
		t.Errorf("expected 1 plugin snapshot imported, got %d", result.PluginImported)
	}

	result2, _ := s.ImportJSON(data)
	if result2.PluginDuped != 1 {
		t.Errorf("expected 1 plugin snapshot duped, got %d", result2.PluginDuped)
	}
}

func TestImportJSON_FullRoundTrip(t *testing.T) {
	// This test verifies that importing a full export with ALL 7 provider types succeeds.
	s := testImportStore(t)
	now := time.Now().UTC().Format(time.RFC3339)

	data := buildExportJSON(t, map[string]interface{}{
		"accounts": []map[string]string{
			{"email": "user@test.com", "plan_name": "Pro", "provider": "antigravity"},
		},
		"subscriptions": []map[string]interface{}{
			{"platform": "GitHub Copilot", "email": "user@test.com", "plan_name": "Pro+", "status": "active", "cost_amount": 39.0, "cost_currency": "USD", "billing_cycle": "monthly"},
		},
		"claudeSnapshots": []map[string]interface{}{
			{"fiveHourPct": 20.0, "capturedAt": now, "source": "statusline"},
		},
		"codexSnapshots": []map[string]interface{}{
			{"accountId": "org-abc", "fiveHourPct": 10.0, "planType": "plus", "capturedAt": now},
		},
		"cursorSnapshots": []map[string]interface{}{
			{"email": "user@cursor.sh", "premiumUsed": 50, "premiumLimit": 500, "usagePct": 10.0, "planType": "pro", "capturedAt": now},
		},
		"geminiSnapshots": []map[string]interface{}{
			{"email": "user@google.com", "tier": "standard", "overallPct": 15.0, "capturedAt": now},
		},
		"copilotSnapshots": []map[string]interface{}{
			{"username": "octocat", "plan": "Pro+", "premiumPct": 40.0, "chatPct": 20.0, "capturedAt": now},
		},
		"pluginSnapshots": []map[string]interface{}{
			{"pluginId": "aws-cost", "provider": "aws", "label": "AWS Cost", "usagePct": 50.0, "capturedAt": now},
		},
	})

	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("full import failed: %v", err)
	}

	if result.AccountsCreated != 1 {
		t.Errorf("accounts: expected 1 created, got %d", result.AccountsCreated)
	}
	if result.SubsCreated != 1 {
		t.Errorf("subs: expected 1 created, got %d", result.SubsCreated)
	}
	if result.ClaudeImported != 1 {
		t.Errorf("claude: expected 1 imported, got %d", result.ClaudeImported)
	}
	if result.CodexImported != 1 {
		t.Errorf("codex: expected 1 imported, got %d", result.CodexImported)
	}
	if result.CursorImported != 1 {
		t.Errorf("cursor: expected 1 imported, got %d", result.CursorImported)
	}
	if result.GeminiImported != 1 {
		t.Errorf("gemini: expected 1 imported, got %d", result.GeminiImported)
	}
	if result.CopilotImported != 1 {
		t.Errorf("copilot: expected 1 imported, got %d", result.CopilotImported)
	}
	if result.PluginImported != 1 {
		t.Errorf("plugin: expected 1 imported, got %d", result.PluginImported)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestImportJSON_SkipsEmptyCapturedAt(t *testing.T) {
	s := testImportStore(t)
	data := buildExportJSON(t, map[string]interface{}{
		"claudeSnapshots": []map[string]interface{}{
			{"fiveHourPct": 10.0, "capturedAt": ""},
		},
		"codexSnapshots": []map[string]interface{}{
			{"accountId": "org-1", "fiveHourPct": 10.0, "capturedAt": ""},
		},
	})
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result.ClaudeImported != 0 || result.CodexImported != 0 {
		t.Errorf("expected 0 imports for empty capturedAt")
	}
}

func TestImportJSON_ParseTimeFlexible(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"2026-05-16T10:30:00Z", true},
		{"2026-05-16T10:30:00+05:30", true},
		{"2026-05-16 10:30:00", true},
		{"not-a-date", false},
		{"", false},
	}
	for _, tt := range tests {
		_, err := parseTimeFlexible(tt.input)
		if (err == nil) != tt.ok {
			t.Errorf("parseTimeFlexible(%q): got err=%v, want ok=%v", tt.input, err, tt.ok)
		}
	}
}

// TestImportJSON_FileRoundTrip tests importing from an actual export file if it exists.
func TestImportJSON_FileRoundTrip(t *testing.T) {
	exportFile := filepath.Join("..", "..", "antigravity_payload_dump.json")
	data, err := os.ReadFile(exportFile)
	if err != nil {
		t.Skipf("skipping file round-trip test: %v", err)
	}

	s := testImportStore(t)
	result, err := s.ImportJSON(data)
	if err != nil {
		t.Fatalf("import from real file failed: %v", err)
	}
	t.Logf("File import result: accounts=%d/%d snaps=%d/%d errs=%d",
		result.AccountsCreated, result.AccountsSkipped,
		result.SnapshotsImported, result.SnapshotsDuped,
		len(result.Errors))
}
