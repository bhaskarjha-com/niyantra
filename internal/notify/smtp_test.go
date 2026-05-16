package notify

import (
	"testing"
)

func TestSMTPConfigIsConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  SMTPConfig
		want bool
	}{
		{
			name: "fully configured",
			cfg:  SMTPConfig{Enabled: true, Host: "smtp.gmail.com", From: "test@example.com", To: "user@example.com"},
			want: true,
		},
		{
			name: "disabled",
			cfg:  SMTPConfig{Enabled: false, Host: "smtp.gmail.com", From: "test@example.com", To: "user@example.com"},
			want: false,
		},
		{
			name: "missing host",
			cfg:  SMTPConfig{Enabled: true, Host: "", From: "test@example.com", To: "user@example.com"},
			want: false,
		},
		{
			name: "missing from",
			cfg:  SMTPConfig{Enabled: true, Host: "smtp.gmail.com", From: "", To: "user@example.com"},
			want: false,
		},
		{
			name: "missing to",
			cfg:  SMTPConfig{Enabled: true, Host: "smtp.gmail.com", From: "test@example.com", To: ""},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSMTPConfigRecipients(t *testing.T) {
	tests := []struct {
		name string
		to   string
		want int
	}{
		{"single", "user@example.com", 1},
		{"multiple", "a@test.com, b@test.com, c@test.com", 3},
		{"with spaces", "  a@test.com , b@test.com  ", 2},
		{"empty", "", 0},
		{"trailing comma", "a@test.com,", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := SMTPConfig{To: tt.to}
			got := cfg.Recipients()
			if len(got) != tt.want {
				t.Errorf("Recipients() len = %d, want %d (got %v)", len(got), tt.want, got)
			}
		})
	}
}

func TestBuildMessage(t *testing.T) {
	msg := buildMessage("from@test.com", []string{"to@test.com"}, "Test Subject", "<p>Hello</p>")

	// Verify headers are present
	s := string(msg)
	checks := []string{
		"From: from@test.com",
		"To: to@test.com",
		"Subject: Test Subject",
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"X-Mailer: Niyantra Dashboard",
		"<p>Hello</p>",
	}
	for _, check := range checks {
		if !contains(s, check) {
			t.Errorf("message missing %q", check)
		}
	}
}

func TestFormatQuotaAlertHTML(t *testing.T) {
	html := FormatQuotaAlertHTML("claude_gpt", 8.5, 10)

	checks := []string{
		"Niyantra",
		"claude_gpt",
		"8.5%",
		"10%",
	}
	for _, check := range checks {
		if !contains(html, check) {
			t.Errorf("quota alert HTML missing %q", check)
		}
	}
}

func TestFormatTestEmailHTML(t *testing.T) {
	html := FormatTestEmailHTML()

	checks := []string{
		"Niyantra",
		"SMTP is working",
		"Test Email",
	}
	for _, check := range checks {
		if !contains(html, check) {
			t.Errorf("test email HTML missing %q", check)
		}
	}
}

func TestSendEmailNotConfigured(t *testing.T) {
	cfg := &SMTPConfig{Enabled: false}
	err := SendEmail(cfg, "test", "body")
	if err == nil {
		t.Error("expected error when not configured")
	}
}

func TestSendEmailNoRecipients(t *testing.T) {
	cfg := &SMTPConfig{Enabled: true, Host: "smtp.test.com", From: "from@test.com", To: ""}
	err := SendEmail(cfg, "test", "body")
	if err == nil {
		t.Error("expected error when no recipients")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexStr(s, substr) >= 0
}

func indexStr(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
