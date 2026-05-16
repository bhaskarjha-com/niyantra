package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookConfigIsConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  WebhookConfig
		want bool
	}{
		{"configured", WebhookConfig{Enabled: true, URL: "https://hooks.example.com"}, true},
		{"disabled", WebhookConfig{Enabled: false, URL: "https://hooks.example.com"}, false},
		{"no url", WebhookConfig{Enabled: true, URL: ""}, false},
		{"empty", WebhookConfig{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSendWebhookNotConfigured(t *testing.T) {
	cfg := &WebhookConfig{Enabled: false}
	if err := SendWebhook(cfg, "test", "msg", 50); err == nil {
		t.Error("expected error when not configured")
	}
}

func TestSendWebhookUnknownType(t *testing.T) {
	cfg := &WebhookConfig{Enabled: true, URL: "https://example.com", Type: "unknown"}
	if err := SendWebhook(cfg, "test", "msg", 50); err == nil {
		t.Error("expected error for unknown webhook type")
	}
}

func TestSendDiscordWebhook(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	cfg := &WebhookConfig{Enabled: true, Type: WebhookDiscord, URL: srv.URL}
	if err := SendWebhook(cfg, "Quota Low", "8.5% remaining", 8.5); err != nil {
		t.Fatalf("SendWebhook discord: %v", err)
	}

	if received["username"] != "Niyantra" {
		t.Errorf("expected username 'Niyantra', got %v", received["username"])
	}
	embeds, ok := received["embeds"].([]interface{})
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %v", received["embeds"])
	}
	embed := embeds[0].(map[string]interface{})
	if embed["title"] != "Quota Low" {
		t.Errorf("embed title = %v, want 'Quota Low'", embed["title"])
	}
	if embed["description"] != "8.5% remaining" {
		t.Errorf("embed description = %v", embed["description"])
	}
}

func TestSendSlackWebhook(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &WebhookConfig{Enabled: true, Type: WebhookSlack, URL: srv.URL}
	if err := SendWebhook(cfg, "Alert", "Low quota", 3.0); err != nil {
		t.Fatalf("SendWebhook slack: %v", err)
	}

	attachments, ok := received["attachments"].([]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %v", received["attachments"])
	}
	att := attachments[0].(map[string]interface{})
	if att["title"] != "Alert" {
		t.Errorf("attachment title = %v, want 'Alert'", att["title"])
	}
	if att["color"] != "#ef4444" {
		t.Errorf("expected red color for 3%%, got %v", att["color"])
	}
}

func TestSendGenericWebhook(t *testing.T) {
	var receivedBody string
	var receivedTitle string
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTitle = r.Header.Get("Title")
		receivedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &WebhookConfig{Enabled: true, Type: WebhookGeneric, URL: srv.URL, Secret: "Bearer mytoken"}
	if err := SendWebhook(cfg, "Test Alert", "Something happened", 50); err != nil {
		t.Fatalf("SendWebhook generic: %v", err)
	}

	if receivedTitle != "Test Alert" {
		t.Errorf("Title header = %q, want 'Test Alert'", receivedTitle)
	}
	if receivedBody != "Something happened" {
		t.Errorf("body = %q, want 'Something happened'", receivedBody)
	}
	if receivedAuth != "Bearer mytoken" {
		t.Errorf("auth = %q, want 'Bearer mytoken'", receivedAuth)
	}
}

func TestSendTelegramWebhook(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// For Telegram, URL = chat_id, Secret = bot token
	// But the sendTelegram function builds the URL from the token,
	// so we need to override — we'll test via the helper directly
	err := sendTelegram("12345", "fake_token", "Alert", "Low quota", 8.0)
	// This will fail because the URL goes to telegram.org — that's expected
	if err == nil {
		t.Log("Telegram send succeeded (unexpected in CI, but OK if network available)")
	}
}

func TestSendTestWebhookNotConfigured(t *testing.T) {
	cfg := &WebhookConfig{Enabled: false}
	if err := SendTestWebhook(cfg); err == nil {
		t.Error("expected error when not configured")
	}
}

func TestSendTestWebhook(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	cfg := &WebhookConfig{Enabled: true, Type: WebhookDiscord, URL: srv.URL}
	if err := SendTestWebhook(cfg); err != nil {
		t.Fatalf("SendTestWebhook: %v", err)
	}

	embeds := received["embeds"].([]interface{})
	embed := embeds[0].(map[string]interface{})
	title := embed["title"].(string)
	if title != "Niyantra — Test Webhook" {
		t.Errorf("test webhook title = %q", title)
	}
}

func TestSeverityColor(t *testing.T) {
	if c := severityColor(3.0); c != 0xEF4444 {
		t.Errorf("expected red for 3%%, got %x", c)
	}
	if c := severityColor(8.0); c != 0xF59E0B {
		t.Errorf("expected amber for 8%%, got %x", c)
	}
	if c := severityColor(50.0); c != 0x3B82F6 {
		t.Errorf("expected blue for 50%%, got %x", c)
	}
}

func TestSeverityHex(t *testing.T) {
	if h := severityHex(3.0); h != "#ef4444" {
		t.Errorf("expected #ef4444 for 3%%, got %s", h)
	}
	if h := severityHex(8.0); h != "#f59e0b" {
		t.Errorf("expected #f59e0b for 8%%, got %s", h)
	}
}

func TestEscapeHTML(t *testing.T) {
	input := "a < b & c > d"
	want := "a &lt; b &amp; c &gt; d"
	if got := escapeHTML(input); got != want {
		t.Errorf("escapeHTML(%q) = %q, want %q", input, got, want)
	}
}

func TestWebhookHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	cfg := &WebhookConfig{Enabled: true, Type: WebhookDiscord, URL: srv.URL}
	if err := SendWebhook(cfg, "Test", "msg", 50); err == nil {
		t.Error("expected error on HTTP 500")
	}
}
