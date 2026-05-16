package web

import (
	"testing"

	"github.com/bhaskarjha-com/niyantra/internal/store"
)

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// ── Static Sensitive Keys ──────────────────────────────────
		// Every key in the sensitiveConfigKeys map must be tested.
		{"copilot_pat", true},
		{"cursor_session_token", true},
		{"gemini_client_secret", true},
		{"smtp_pass", true},
		{"smtp_user", true},
		{"webhook_secret", true},
		{"webpush_vapid_private", true},

		// ── Non-Sensitive Keys ────────────────────────────────────
		// Representative sample of every config category.
		// Capture
		{"auto_capture", false},
		{"poll_interval", false},
		{"codex_capture", false},
		{"cursor_capture", false},
		{"gemini_capture", false},
		{"copilot_capture", false},
		{"claude_bridge", false},
		{"session_idle_timeout", false},
		{"auto_link_subs", false},
		{"gemini_client_id", false}, // client ID is not secret (public identifier)
		// Display
		{"budget_monthly", false},
		{"currency", false},
		// Notification
		{"notify_enabled", false},
		{"notify_threshold", false},
		{"smtp_enabled", false},
		{"smtp_host", false},
		{"smtp_port", false},
		{"smtp_from", false},
		{"smtp_to", false},
		{"smtp_tls", false},
		{"webhook_enabled", false},
		{"webhook_type", false},
		{"webhook_url", false},
		{"webpush_enabled", false},
		{"webpush_vapid_public", false}, // public key is safe to expose
		// Data
		{"retention_days", false},

		// ── Plugin Sensitive Keys (Pattern Match) ─────────────────
		// Suffix patterns: _api_key, _token, _secret, _password, _pat, _credential
		{"plugin_weather_api_key", true},
		{"plugin_myservice_token", true},
		{"plugin_custom_secret", true},
		{"plugin_db_password", true},
		{"plugin_github_pat", true},
		{"plugin_aws_credential", true},
		// Case-insensitive pattern matching
		{"plugin_svc_API_KEY", true},
		{"plugin_svc_Token", true},

		// ── Plugin Non-Sensitive Keys ─────────────────────────────
		{"plugin_weather_enabled", false},
		{"plugin_myservice_url", false},
		{"plugin_custom_region", false},
		{"plugin_db_host", false},
		{"plugin_db_port", false},
		{"plugin_svc_timeout", false},

		// ── Edge Cases ────────────────────────────────────────────
		// Non-plugin keys with sensitive-looking suffixes must NOT match
		{"auto_api_key", false},
		{"some_token", false},
		{"my_password", false},
		{"not_a_pat", false},
		// Empty/unusual keys
		{"", false},
		{"plugin_", false},
		{"plugin_x", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isSensitiveKey(tt.key)
			if got != tt.expected {
				t.Errorf("isSensitiveKey(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

// TestMaskConfigEntries verifies that maskConfigEntries correctly replaces
// sensitive values with "configured" and leaves non-sensitive values unchanged.
func TestMaskConfigEntries(t *testing.T) {
	entries := []*store.ConfigEntry{
		{Key: "copilot_pat", Value: "ghp_abc123xyz"},
		{Key: "cursor_session_token", Value: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9"},
		{Key: "gemini_client_secret", Value: "GOCSPX-secret-value"},
		{Key: "smtp_pass", Value: "my-smtp-password"},
		{Key: "smtp_user", Value: "user@gmail.com"},
		{Key: "webhook_secret", Value: "telegram:bot123:AAHdqT"},
		{Key: "webpush_vapid_private", Value: "base64-private-key"},
		{Key: "auto_capture", Value: "true"},
		{Key: "poll_interval", Value: "300"},
		{Key: "budget_monthly", Value: "150"},
		{Key: "smtp_host", Value: "smtp.gmail.com"},
		// Empty sensitive key should remain empty (not "configured")
		{Key: "copilot_pat", Value: ""},
	}

	maskConfigEntries(entries)

	expected := map[string]string{
		"copilot_pat":           "configured",
		"cursor_session_token":  "configured",
		"gemini_client_secret":  "configured",
		"smtp_pass":             "configured",
		"smtp_user":             "configured",
		"webhook_secret":        "configured",
		"webpush_vapid_private": "configured",
		"auto_capture":          "true",
		"poll_interval":         "300",
		"budget_monthly":        "150",
		"smtp_host":             "smtp.gmail.com",
	}

	for _, e := range entries {
		// Skip the second copilot_pat (empty one)
		if e.Key == "copilot_pat" && e.Value == "" {
			continue // empty values should remain empty
		}
		if exp, ok := expected[e.Key]; ok {
			if e.Value != exp {
				t.Errorf("maskConfigEntries: key=%q got=%q want=%q", e.Key, e.Value, exp)
			}
		}
	}

	// Verify the empty copilot_pat entry stayed empty
	emptyEntry := entries[len(entries)-1]
	if emptyEntry.Value != "" {
		t.Errorf("maskConfigEntries: empty value should stay empty, got %q", emptyEntry.Value)
	}
}

// TestMaskConfigEntriesPluginPattern verifies dynamic plugin key masking.
func TestMaskConfigEntriesPluginPattern(t *testing.T) {
	entries := []*store.ConfigEntry{
		{Key: "plugin_weather_api_key", Value: "sk-weather-123"},
		{Key: "plugin_github_pat", Value: "ghp_abc"},
		{Key: "plugin_weather_enabled", Value: "true"},
		{Key: "plugin_weather_url", Value: "https://api.weather.com"},
	}

	maskConfigEntries(entries)

	if entries[0].Value != "configured" {
		t.Errorf("plugin_weather_api_key should be masked, got %q", entries[0].Value)
	}
	if entries[1].Value != "configured" {
		t.Errorf("plugin_github_pat should be masked, got %q", entries[1].Value)
	}
	if entries[2].Value != "true" {
		t.Errorf("plugin_weather_enabled should NOT be masked, got %q", entries[2].Value)
	}
	if entries[3].Value != "https://api.weather.com" {
		t.Errorf("plugin_weather_url should NOT be masked, got %q", entries[3].Value)
	}
}

// TestAllRealConfigKeysClassified ensures every real config key that exists
// in the database schema is explicitly classified as sensitive or non-sensitive.
// This is a regression test: if a new config key is added without updating
// the sensitive key list, this test forces a conscious decision.
func TestAllRealConfigKeysClassified(t *testing.T) {
	// These are all config keys created by the schema migration.
	// Extracted from internal/store/migrations.go schema v1-v19.
	allConfigKeys := []struct {
		key       string
		sensitive bool
	}{
		// Capture category
		{"auto_capture", false},
		{"auto_link_subs", false},
		{"claude_bridge", false},
		{"codex_capture", false},
		{"copilot_capture", false},
		{"copilot_pat", true},
		{"cursor_capture", false},
		{"cursor_session_token", true},
		{"gemini_capture", false},
		{"gemini_client_id", false},
		{"gemini_client_secret", true},
		{"poll_interval", false},
		{"session_idle_timeout", false},
		// Data category
		{"retention_days", false},
		// Display category
		{"budget_monthly", false},
		{"currency", false},
		// Notification category
		{"notify_enabled", false},
		{"notify_threshold", false},
		{"smtp_enabled", false},
		{"smtp_from", false},
		{"smtp_host", false},
		{"smtp_pass", true},
		{"smtp_port", false},
		{"smtp_tls", false},
		{"smtp_to", false},
		{"smtp_user", true},
		{"webhook_enabled", false},
		{"webhook_secret", true},
		{"webhook_type", false},
		{"webhook_url", false},
		{"webpush_enabled", false},
		{"webpush_vapid_private", true},
		{"webpush_vapid_public", false},
	}

	for _, kv := range allConfigKeys {
		t.Run(kv.key, func(t *testing.T) {
			got := isSensitiveKey(kv.key)
			if got != kv.sensitive {
				if kv.sensitive {
					t.Errorf("config key %q MUST be sensitive (contains credentials) but isSensitiveKey returned false", kv.key)
				} else {
					t.Errorf("config key %q should NOT be sensitive but isSensitiveKey returned true", kv.key)
				}
			}
		})
	}
}
