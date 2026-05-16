package agent

import (
	"context"
	"errors"

	"github.com/bhaskarjha-com/niyantra/internal/copilot"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// pollCopilot polls GitHub Copilot usage if copilot_capture is enabled.
func (a *PollingAgent) pollCopilot(ctx context.Context) {
	if !a.store.GetConfigBool("copilot_capture") {
		return
	}

	pat := a.store.GetConfig("copilot_pat")
	if pat == "" {
		return
	}

	if a.copilotAuthFails >= 3 {
		a.logger.Debug("Copilot polling paused (auth failures)", "failures", a.copilotAuthFails)
		return
	}

	client := copilot.NewClient(pat, a.logger)
	snapshot, err := client.FetchSnapshot(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, copilot.ErrUnauthorized) || errors.Is(err, copilot.ErrForbidden) {
			a.copilotAuthFails++
			a.logger.Warn("Copilot auth error", "error", err, "failures", a.copilotAuthFails)
		} else {
			a.logger.Warn("Copilot poll failed", "error", err)
		}
		return
	}

	a.copilotAuthFails = 0

	snap := &store.CopilotSnapshot{
		Email:         snapshot.Email,
		Username:      snapshot.Username,
		Plan:          snapshot.Plan,
		PremiumPct:    snapshot.PremiumPct,
		ChatPct:       snapshot.ChatPct,
		HasPremium:    snapshot.HasPremium,
		HasChat:       snapshot.HasChat,
		CaptureMethod: "auto",
		CaptureSource: "server",
	}

	// Create/update account if we have identity info
	email := snapshot.Email
	if email == "" && snapshot.Username != "" {
		email = snapshot.Username + "@github.com"
	}
	if email != "" {
		accountID, err := a.store.GetOrCreateAccount(email, "Copilot "+snapshot.Plan, "copilot")
		if err == nil {
			snap.AccountID = accountID
		}
	}

	if _, err := a.store.InsertCopilotSnapshot(snap); err != nil {
		a.logger.Error("Failed to store Copilot snapshot", "error", err)
		return
	}

	a.store.UpdateSourceCapture("copilot")

	if a.notifier != nil {
		a.notifier.CheckClaudeQuota("copilot_usage", snapshot.PremiumPct)
	}

	a.logger.Info("Copilot poll complete",
		"plan", snapshot.Plan, "premiumPct", snapshot.PremiumPct,
		"chatPct", snapshot.ChatPct, "username", snapshot.Username)
}
