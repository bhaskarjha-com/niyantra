package notify

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Engine tracks notification state and prevents spam.
// Fires at most one notification per model per reset cycle.
// Supports dual-channel delivery: OS-native + SMTP email (F11).
type Engine struct {
	mu        sync.Mutex
	enabled   bool
	threshold float64         // alert when remaining% drops below this (default 10)
	guard     map[string]bool // model → has been notified this cycle
	logger    *slog.Logger
	smtp      SMTPConfig      // F11: SMTP email delivery settings

	onNotify func(model string, remainingPct float64) // callback when notification fires
}

// NewEngine creates a notification engine with default settings.
func NewEngine(logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		enabled:   false,
		threshold: 10,
		guard:     make(map[string]bool),
		logger:    logger,
	}
}

// Configure updates the engine's settings.
func (e *Engine) Configure(enabled bool, threshold float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = enabled
	if threshold > 0 {
		e.threshold = threshold
	}
}

// ConfigureSMTP updates the SMTP delivery settings (F11).
func (e *Engine) ConfigureSMTP(cfg SMTPConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.smtp = cfg
	e.logger.Info("SMTP notification channel configured",
		"enabled", cfg.Enabled,
		"host", cfg.Host,
		"port", cfg.Port,
		"tls", cfg.TLSMode)
}

// SMTPEnabled returns whether SMTP delivery is active.
func (e *Engine) SMTPEnabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.smtp.IsConfigured()
}

// Enabled returns whether notifications are enabled.
func (e *Engine) Enabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enabled
}

// Threshold returns the current threshold.
func (e *Engine) Threshold() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.threshold
}

// SetOnNotify registers a callback invoked after a notification is successfully sent.
// Used by the server to create system_alerts and log activity.
func (e *Engine) SetOnNotify(fn func(model string, remainingPct float64)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onNotify = fn
}

// CheckQuota fires a notification if remaining% drops below threshold.
// remainingPct is the percentage of quota remaining (0-100).
// Guard: fires at most once per model until OnReset() is called.
func (e *Engine) CheckQuota(model string, remainingPct float64) {
	e.mu.Lock()
	enabled := e.enabled
	threshold := e.threshold
	alreadySent := e.guard[model]
	e.mu.Unlock()

	if !enabled || alreadySent {
		return
	}

	if remainingPct > threshold {
		return
	}

	// Fire notification
	title := fmt.Sprintf("⚠️ %s quota low", model)
	body := fmt.Sprintf("%.1f%% remaining — consider switching models", remainingPct)

	e.logger.Info("Sending quota alert notification",
		"model", model,
		"remaining_pct", remainingPct,
		"threshold", threshold)

	// Channel 1: OS-native desktop notification
	if err := Send(title, body); err != nil {
		e.logger.Error("Failed to send OS notification", "error", err, "model", model)
		// Continue — email may still succeed
	}

	// Channel 2: SMTP email notification (F11)
	e.mu.Lock()
	smtpCfg := e.smtp
	e.mu.Unlock()

	if smtpCfg.IsConfigured() {
		go func() {
			subject := fmt.Sprintf("Niyantra Alert: %s quota low (%.1f%%)", model, remainingPct)
			htmlBody := FormatQuotaAlertHTML(model, remainingPct, threshold)
			if err := SendEmail(&smtpCfg, subject, htmlBody); err != nil {
				e.logger.Error("Failed to send SMTP notification", "error", err, "model", model)
			} else {
				e.logger.Info("SMTP quota alert sent", "model", model, "to", smtpCfg.To)
			}
		}()
	}

	// Mark as notified for this cycle
	e.mu.Lock()
	e.guard[model] = true
	cb := e.onNotify
	e.mu.Unlock()

	// Fire the notification callback (system alert + activity log)
	if cb != nil {
		cb(model, remainingPct)
	}
}

// CheckClaudeQuota fires a notification for Claude Code rate limits.
// usedPct is the used percentage (0-100).
func (e *Engine) CheckClaudeQuota(window string, usedPct float64) {
	remaining := 100.0 - usedPct
	key := "claude_" + window
	e.CheckQuota(key, remaining)
}

// OnReset clears the guard for a model (cycle detected → can notify again).
func (e *Engine) OnReset(model string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.guard, model)
	e.logger.Debug("Notification guard cleared for model", "model", model)
}

// SendTest sends a test notification to verify the platform works.
func (e *Engine) SendTest() error {
	return Send(
		"Niyantra — Test Notification",
		fmt.Sprintf("Notifications are working! Threshold: %.0f%%. Time: %s",
			e.Threshold(), time.Now().Format("15:04:05")),
	)
}

// SendTestEmail sends a test email to verify SMTP configuration (F11).
func (e *Engine) SendTestEmail() error {
	e.mu.Lock()
	cfg := e.smtp
	e.mu.Unlock()

	if !cfg.IsConfigured() {
		return fmt.Errorf("SMTP is not configured")
	}

	return SendEmail(&cfg, "Niyantra — SMTP Test", FormatTestEmailHTML())
}
