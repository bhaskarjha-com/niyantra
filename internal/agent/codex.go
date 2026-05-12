package agent

import (
	"context"
	"errors"

	"github.com/bhaskarjha-com/niyantra/internal/codex"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"time"
)

// pollCodex polls Codex usage if codex_capture is enabled.
// Runs alongside Antigravity on each tick with independent auth backoff.
func (a *PollingAgent) pollCodex(ctx context.Context) {
	if !a.store.GetConfigBool("codex_capture") {
		return
	}

	// Auth failure backoff (independent of Antigravity)
	if a.codexAuthFails >= 3 {
		a.logger.Debug("Codex polling paused (auth failures)", "failures", a.codexAuthFails)
		return
	}

	// Detect credentials and create/refresh client
	creds, err := codex.DetectCredentials(a.logger)
	if err != nil {
		if errors.Is(err, codex.ErrNotInstalled) {
			return // Codex not installed, silently skip
		}
		a.logger.Debug("Codex credential detection failed", "error", err)
		return
	}

	// Proactive token refresh: if token expires within 6 hours
	if creds.IsExpiringSoon(6*time.Hour) && creds.RefreshToken != "" {
		a.logger.Info("Codex token expiring soon, refreshing")
		newTokens, err := codex.RefreshToken(ctx, creds.RefreshToken)
		if err != nil {
			if errors.Is(err, codex.ErrRefreshTokenReused) {
				a.codexAuthFails = 3 // permanent pause until re-auth
				a.logger.Error("Codex refresh token reused — re-authenticate via 'codex auth'")
				return
			}
			a.logger.Warn("Codex token refresh failed", "error", err)
		} else {
			// CRITICAL: Save new tokens immediately (rotation = one-time-use)
			if err := codex.WriteCredentials(newTokens.AccessToken, newTokens.RefreshToken, newTokens.IDToken); err != nil {
				a.logger.Error("Failed to save refreshed Codex credentials", "error", err)
			} else {
				creds.AccessToken = newTokens.AccessToken
				a.logger.Info("Codex token refreshed")
			}
		}
	}

	// Create client with current token
	if a.codexClient == nil {
		a.codexClient = codex.NewClient(creds.AccessToken, creds.AccountID, a.logger)
	}
	a.codexClient.SetToken(creds.AccessToken)

	// Fetch usage
	usage, err := a.codexClient.FetchUsage(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, codex.ErrUnauthorized) || errors.Is(err, codex.ErrForbidden) {
			a.codexAuthFails++
			a.logger.Warn("Codex auth error", "error", err, "failures", a.codexAuthFails)
		} else {
			a.logger.Warn("Codex poll failed", "error", err)
		}
		return
	}

	// Success — reset auth failures
	a.codexAuthFails = 0

	// Build and store snapshot
	snap := &store.CodexSnapshot{
		AccountID:      creds.AccountID,
		Email:          creds.Email,
		FiveHourPct:    0,
		PlanType:       usage.PlanType,
		CreditsBalance: usage.CreditsBalance,
		CaptureMethod:  "auto",
		CaptureSource:  "server",
	}

	for _, q := range usage.Quotas {
		switch q.Name {
		case "five_hour":
			snap.FiveHourPct = q.Utilization
			snap.FiveHourReset = q.ResetsAt
		case "seven_day":
			v := q.Utilization
			snap.SevenDayPct = &v
			snap.SevenDayReset = q.ResetsAt
		case "code_review":
			v := q.Utilization
			snap.CodeReviewPct = &v
		}
	}

	if _, err := a.store.InsertCodexSnapshot(snap); err != nil {
		a.logger.Error("Failed to store Codex snapshot", "error", err)
		return
	}

	// Update data source bookkeeping
	a.store.UpdateSourceCapture("codex")

	// F9: Check Codex notification thresholds
	if a.notifier != nil {
		a.notifier.CheckClaudeQuota("codex_five_hour", snap.FiveHourPct)
		if snap.SevenDayPct != nil {
			a.notifier.CheckClaudeQuota("codex_seven_day", *snap.SevenDayPct)
		}
	}

	// Feed session manager
	if a.codexSM != nil {
		var vals []float64
		for _, q := range usage.Quotas {
			vals = append(vals, q.Utilization)
		}
		a.codexSM.ReportPoll(vals)
	}

	a.logger.Info("Codex poll complete",
		"plan", usage.PlanType,
		"quotas", len(usage.Quotas),
		"account_id", creds.AccountID)
}
