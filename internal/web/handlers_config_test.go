package web

import (
	"testing"
)

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// Static sensitive keys
		{"copilot_pat", true},
		{"smtp_pass", true},
		{"smtp_user", true},
		{"webhook_secret", true},
		{"webpush_vapid_private", true},

		// Non-sensitive keys
		{"auto_capture", false},
		{"poll_interval", false},
		{"budget_monthly", false},
		{"notify_enabled", false},
		{"webpush_vapid_public", false},

		// Plugin sensitive keys (pattern match)
		{"plugin_weather_api_key", true},
		{"plugin_myservice_token", true},
		{"plugin_custom_secret", true},
		{"plugin_db_password", true},
		{"plugin_github_pat", true},
		{"plugin_aws_credential", true},

		// Plugin non-sensitive keys (should NOT match)
		{"plugin_weather_enabled", false},
		{"plugin_myservice_url", false},
		{"plugin_custom_region", false},
		{"plugin_db_host", false},

		// Edge cases: non-plugin keys with sensitive-looking suffixes
		{"auto_api_key", false},     // not a plugin_ prefix
		{"some_token", false},       // not a plugin_ prefix
		{"my_password", false},      // not a plugin_ prefix
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
