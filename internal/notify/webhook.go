package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebhookType defines the supported webhook integrations.
type WebhookType string

const (
	WebhookDiscord  WebhookType = "discord"
	WebhookTelegram WebhookType = "telegram"
	WebhookSlack    WebhookType = "slack"
	WebhookGeneric  WebhookType = "generic" // ntfy, Gotify, custom
)

// WebhookConfig holds webhook delivery settings.
type WebhookConfig struct {
	Enabled bool        // Master toggle
	Type    WebhookType // discord, telegram, slack, generic
	URL     string      // Webhook URL (Discord/Slack) or API base (Telegram)
	Secret  string      // Bot token (Telegram), auth header (generic), unused (Discord/Slack)
}

// IsConfigured returns true if the minimum webhook settings are present.
func (c *WebhookConfig) IsConfigured() bool {
	return c.Enabled && c.URL != ""
}

// webhookHTTPClient is a shared HTTP client for webhook deliveries.
var webhookHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

// SendWebhook dispatches a notification to the configured webhook service.
func SendWebhook(cfg *WebhookConfig, title, message string, remainingPct float64) error {
	if !cfg.IsConfigured() {
		return fmt.Errorf("webhook: not configured")
	}

	switch cfg.Type {
	case WebhookDiscord:
		return sendDiscord(cfg.URL, title, message, remainingPct)
	case WebhookTelegram:
		return sendTelegram(cfg.URL, cfg.Secret, title, message, remainingPct)
	case WebhookSlack:
		return sendSlack(cfg.URL, title, message, remainingPct)
	case WebhookGeneric:
		return sendGeneric(cfg.URL, cfg.Secret, title, message, remainingPct)
	default:
		return fmt.Errorf("webhook: unknown type %q", cfg.Type)
	}
}

// ── Discord ───────────────────────────────────────────────────────

func sendDiscord(webhookURL, title, message string, remainingPct float64) error {
	color := severityColor(remainingPct)
	payload := map[string]interface{}{
		"username":   "Niyantra",
		"avatar_url": "https://raw.githubusercontent.com/bhaskarjha-com/niyantra/main/docs/icon.png",
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": message,
				"color":       color,
				"footer": map[string]string{
					"text": "Niyantra — AI Operations Dashboard",
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
	return postJSON(webhookURL, "", payload)
}

// ── Telegram ──────────────────────────────────────────────────────

func sendTelegram(chatID, botToken, title, message string, remainingPct float64) error {
	if botToken == "" {
		return fmt.Errorf("webhook: telegram bot token required")
	}

	severity := "⚠️"
	if remainingPct < 5 {
		severity = "🔴"
	}

	text := fmt.Sprintf(
		"%s <b>%s</b>\n\n%s\n\n<i>Niyantra — AI Operations Dashboard</i>",
		severity, escapeHTML(title), escapeHTML(message),
	)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	return postJSON(url, "", payload)
}

// ── Slack ─────────────────────────────────────────────────────────

func sendSlack(webhookURL, title, message string, remainingPct float64) error {
	color := severityHex(remainingPct)
	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  title,
				"text":   message,
				"footer": "Niyantra — AI Operations Dashboard",
				"ts":     time.Now().Unix(),
			},
		},
	}
	return postJSON(webhookURL, "", payload)
}

// ── Generic (ntfy, Gotify, custom) ───────────────────────────────

func sendGeneric(url, authHeader, title, message string, _ float64) error {
	req, err := http.NewRequest("POST", url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("webhook: create request: %w", err)
	}

	// ntfy-compatible headers
	req.Header.Set("Title", title)
	req.Header.Set("Priority", "high")
	req.Header.Set("Tags", "warning")

	// Optional auth header (e.g., "Bearer xxx" or "Basic xxx")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook: HTTP %d from %s", resp.StatusCode, url)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────

// postJSON sends a JSON payload via POST.
func postJSON(url, authHeader string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshal: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook: HTTP %d from %s", resp.StatusCode, url)
	}
	return nil
}

// severityColor returns a Discord embed color (decimal int) based on remaining%.
func severityColor(remainingPct float64) int {
	if remainingPct < 5 {
		return 0xEF4444 // red
	}
	if remainingPct < 10 {
		return 0xF59E0B // amber
	}
	return 0x3B82F6 // blue
}

// severityHex returns a Slack attachment color hex based on remaining%.
func severityHex(remainingPct float64) string {
	if remainingPct < 5 {
		return "#ef4444"
	}
	if remainingPct < 10 {
		return "#f59e0b"
	}
	return "#3b82f6"
}

// escapeHTML escapes special chars for Telegram HTML parse mode.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// SendTestWebhook sends a test notification to verify webhook config.
func SendTestWebhook(cfg *WebhookConfig) error {
	if !cfg.IsConfigured() {
		return fmt.Errorf("webhook: not configured")
	}
	return SendWebhook(
		cfg,
		"Niyantra — Test Webhook",
		fmt.Sprintf("✅ Webhook notifications are working!\nType: %s\nTime: %s", cfg.Type, time.Now().Format("15:04:05")),
		50.0, // neutral — just a test
	)
}
