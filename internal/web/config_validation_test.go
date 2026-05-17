package web

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// testServerWithStore creates a minimal Server with a real store for config validation testing.
func testServerWithStore(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "config_validation_test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	srv := &Server{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		store:  s,
	}
	return srv
}

func TestValidateConfigValue_Bool(t *testing.T) {
	srv := testServerWithStore(t)

	tests := []struct {
		key   string
		value string
		ok    bool
	}{
		{"auto_capture", "true", true},
		{"auto_capture", "false", true},
		{"auto_capture", "yes", false},
		{"auto_capture", "1", false},
		{"auto_capture", "", false},
		{"notify_enabled", "true", true},
		{"notify_enabled", "banana", false},
	}

	for _, tt := range tests {
		errMsg := srv.validateConfigValue(tt.key, tt.value)
		if tt.ok && errMsg != "" {
			t.Errorf("validateConfigValue(%q, %q) = %q, want ok", tt.key, tt.value, errMsg)
		}
		if !tt.ok && errMsg == "" {
			t.Errorf("validateConfigValue(%q, %q) = ok, want error", tt.key, tt.value)
		}
	}
}

func TestValidateConfigValue_Int(t *testing.T) {
	srv := testServerWithStore(t)

	tests := []struct {
		key   string
		value string
		ok    bool
	}{
		{"poll_interval", "300", true},
		{"poll_interval", "30", true},
		{"poll_interval", "3600", true},
		{"poll_interval", "29", false},  // below minimum
		{"poll_interval", "3601", false}, // above maximum
		{"poll_interval", "abc", false},  // not a number
		{"retention_days", "365", true},
		{"retention_days", "30", true},
		{"retention_days", "29", false},
		{"retention_days", "3651", false},
	}

	for _, tt := range tests {
		errMsg := srv.validateConfigValue(tt.key, tt.value)
		if tt.ok && errMsg != "" {
			t.Errorf("validateConfigValue(%q, %q) = %q, want ok", tt.key, tt.value, errMsg)
		}
		if !tt.ok && errMsg == "" {
			t.Errorf("validateConfigValue(%q, %q) = ok, want error", tt.key, tt.value)
		}
	}
}

func TestValidateConfigValue_StringPassesThrough(t *testing.T) {
	srv := testServerWithStore(t)

	// String-type config keys should accept any value
	tests := []struct {
		key   string
		value string
	}{
		{"antigravity_url", "https://example.com"},
		{"antigravity_url", ""},
		{"smtp_host", "mail.example.com"},
	}

	for _, tt := range tests {
		errMsg := srv.validateConfigValue(tt.key, tt.value)
		if errMsg != "" {
			t.Errorf("validateConfigValue(%q, %q) = %q, want ok (string type should pass)", tt.key, tt.value, errMsg)
		}
	}
}

func TestValidateConfigValue_UnknownKey(t *testing.T) {
	srv := testServerWithStore(t)
	// Unknown keys should pass validation (validation is only for known types)
	errMsg := srv.validateConfigValue("nonexistent_key_12345", "anything")
	if errMsg != "" {
		t.Errorf("unknown key should pass validation, got %q", errMsg)
	}
}

func TestValidateConfigValue_NotifyThreshold(t *testing.T) {
	srv := testServerWithStore(t)

	tests := []struct {
		value string
		ok    bool
	}{
		{"10", true},
		{"5", true},
		{"50", true},
		{"4", false},
		{"51", false},
		{"abc", false},
	}

	for _, tt := range tests {
		errMsg := srv.validateConfigValue("notify_threshold", tt.value)
		if tt.ok && errMsg != "" {
			t.Errorf("notify_threshold=%q: %q, want ok", tt.value, errMsg)
		}
		if !tt.ok && errMsg == "" {
			t.Errorf("notify_threshold=%q: ok, want error", tt.value)
		}
	}
}
