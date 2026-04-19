package notify

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Engine tracks notification state and prevents spam.
// Fires at most one notification per model per reset cycle.
type Engine struct {
	mu        sync.Mutex
	enabled   bool
	threshold float64            // alert when remaining% drops below this (default 10)
	guard     map[string]bool    // model → has been notified this cycle
	logger    *slog.Logger
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

	if err := Send(title, body); err != nil {
		e.logger.Error("Failed to send notification", "error", err, "model", model)
		return
	}

	// Mark as notified for this cycle
	e.mu.Lock()
	e.guard[model] = true
	e.mu.Unlock()
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
