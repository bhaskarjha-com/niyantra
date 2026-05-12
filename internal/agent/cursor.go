package agent

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bhaskarjha-com/niyantra/internal/cursor"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// cursorAuthFails tracks consecutive auth failures for cursor (independent backoff).
// This is stored as a field on PollingAgent — see agent.go struct fields added below.

// pollCursor polls Cursor usage if cursor_capture is enabled.
// Runs alongside other providers on each tick with independent auth backoff.
func (a *PollingAgent) pollCursor(ctx context.Context) {
	if !a.store.GetConfigBool("cursor_capture") {
		return
	}

	// Auth failure backoff (independent of other providers)
	if a.cursorAuthFails >= 3 {
		a.logger.Debug("Cursor polling paused (auth failures)", "failures", a.cursorAuthFails)
		return
	}

	// Detect credentials (auto from state.vscdb or manual from config)
	manualToken := a.store.GetConfig("cursor_session_token")
	creds, err := cursor.DetectCredentials(a.logger, manualToken)
	if err != nil {
		if errors.Is(err, cursor.ErrNotInstalled) {
			return // Cursor not installed, silently skip
		}
		a.logger.Debug("Cursor credential detection failed", "error", err)
		return
	}

	// Create client and fetch usage
	client := cursor.NewClient(creds.AccessToken, a.logger)
	usage, err := client.FetchUsage(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, cursor.ErrUnauthorized) || errors.Is(err, cursor.ErrForbidden) {
			a.cursorAuthFails++
			a.logger.Warn("Cursor auth error", "error", err, "failures", a.cursorAuthFails)
		} else {
			a.logger.Warn("Cursor poll failed", "error", err)
		}
		return
	}

	// Success — reset auth failures
	a.cursorAuthFails = 0

	// Build models JSON
	modelsJSON, _ := json.Marshal(usage.Models)

	// Build and store snapshot
	snap := &store.CursorSnapshot{
		Email:         creds.Email,
		PremiumUsed:   usage.PremiumUsed,
		PremiumLimit:  usage.PremiumLimit,
		UsagePct:      usage.UsagePct(),
		StartOfMonth:  usage.StartOfMonth,
		ModelsJSON:    string(modelsJSON),
		CaptureMethod: "auto",
		CaptureSource: "server",
	}

	// Link to account if email is known
	if creds.Email != "" {
		accountID, err := a.store.GetOrCreateAccount(creds.Email, "Cursor", "cursor")
		if err == nil {
			snap.AccountID = accountID
		}
	}

	if _, err := a.store.InsertCursorSnapshot(snap); err != nil {
		a.logger.Error("Failed to store Cursor snapshot", "error", err)
		return
	}

	// Update data source bookkeeping
	a.store.UpdateSourceCapture("cursor")

	// F9: Check Cursor notification thresholds
	if a.notifier != nil {
		a.notifier.CheckClaudeQuota("cursor_usage", usage.UsagePct())
	}

	a.logger.Info("Cursor poll complete",
		"premiumUsed", usage.PremiumUsed,
		"premiumLimit", usage.PremiumLimit,
		"usagePct", usage.UsagePct(),
		"models", len(usage.Models))
}
