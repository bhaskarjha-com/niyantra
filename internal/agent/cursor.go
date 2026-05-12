package agent

import (
	"context"
	"errors"

	"github.com/bhaskarjha-com/niyantra/internal/cursor"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// pollCursor polls Cursor usage if cursor_capture is enabled.
func (a *PollingAgent) pollCursor(ctx context.Context) {
	if !a.store.GetConfigBool("cursor_capture") {
		return
	}

	if a.cursorAuthFails >= 3 {
		a.logger.Debug("Cursor polling paused (auth failures)", "failures", a.cursorAuthFails)
		return
	}

	manualToken := a.store.GetConfig("cursor_session_token")
	creds, err := cursor.DetectCredentials(a.logger, manualToken)
	if err != nil {
		if errors.Is(err, cursor.ErrNotInstalled) || errors.Is(err, cursor.ErrNoToken) {
			return
		}
		a.logger.Debug("Cursor credential detection failed", "error", err)
		return
	}

	client := cursor.NewClient(creds, a.logger)
	snapshot, err := client.FetchSnapshot(ctx)
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

	a.cursorAuthFails = 0

	snap := &store.CursorSnapshot{
		Email:         creds.Email,
		BillingModel:  snapshot.BillingModel,
		PlanTier:      snapshot.PlanTier,
		RequestsUsed:  snapshot.RequestsUsed,
		RequestsMax:   snapshot.RequestsMax,
		UsedCents:     snapshot.UsedCents,
		LimitCents:    snapshot.LimitCents,
		UsagePct:      snapshot.UsagePct(),
		AutoPct:       snapshot.AutoPercentUsed,
		APIPct:        snapshot.APIPercentUsed,
		CycleStart:    snapshot.CycleStart,
		CycleEnd:      snapshot.CycleEnd,
		CaptureMethod: "auto",
		CaptureSource: "server",
	}

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

	a.store.UpdateSourceCapture("cursor")

	if a.notifier != nil {
		a.notifier.CheckClaudeQuota("cursor_usage", snapshot.UsagePct())
	}

	a.logger.Info("Cursor poll complete",
		"billing", snapshot.BillingModel, "plan", snapshot.PlanTier,
		"usagePct", snapshot.UsagePct())
}
