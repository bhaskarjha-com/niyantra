package agent

import (
	"context"
	"errors"

	"github.com/bhaskarjha-com/niyantra/internal/gemini"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// pollGemini polls Gemini CLI usage if gemini_capture is enabled.
func (a *PollingAgent) pollGemini(ctx context.Context) {
	if !a.store.GetConfigBool("gemini_capture") {
		return
	}

	if a.geminiAuthFails >= 3 {
		a.logger.Debug("Gemini polling paused (auth failures)", "failures", a.geminiAuthFails)
		return
	}

	creds, err := gemini.DetectCredentials(a.logger)
	if err != nil {
		if errors.Is(err, gemini.ErrNotInstalled) || errors.Is(err, gemini.ErrNoToken) {
			return
		}
		a.logger.Debug("Gemini credential detection failed", "error", err)
		return
	}

	// Get OAuth client creds for token refresh
	clientID := a.store.GetConfig("gemini_client_id")
	clientSecret := a.store.GetConfig("gemini_client_secret")
	if clientID == "" || clientSecret == "" {
		// Try auto-extraction from Gemini CLI installation
		clientID, clientSecret = gemini.ExtractOAuthClientCreds(a.logger)
	}

	client := gemini.NewClient(creds, a.logger, clientID, clientSecret)
	snapshot, err := client.FetchSnapshot(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, gemini.ErrUnauthorized) || errors.Is(err, gemini.ErrForbidden) || errors.Is(err, gemini.ErrTokenExpired) {
			a.geminiAuthFails++
			a.logger.Warn("Gemini auth error", "error", err, "failures", a.geminiAuthFails)
		} else {
			a.logger.Warn("Gemini poll failed", "error", err)
		}
		return
	}

	a.geminiAuthFails = 0

	snap := &store.GeminiSnapshot{
		Email:         snapshot.Email,
		Tier:          snapshot.Tier,
		OverallPct:    snapshot.OverallUsedPct,
		ModelsJSON:    store.FormatGeminiModelsJSON(snapshot.Models),
		ProjectID:     snapshot.ProjectID,
		CaptureMethod: "auto",
		CaptureSource: "server",
	}

	if snapshot.Email != "" {
		accountID, err := a.store.GetOrCreateAccount(snapshot.Email, "Gemini CLI", "gemini")
		if err == nil {
			snap.AccountID = accountID
		}
	}

	if _, err := a.store.InsertGeminiSnapshot(snap); err != nil {
		a.logger.Error("Failed to store Gemini snapshot", "error", err)
		return
	}

	a.store.UpdateSourceCapture("gemini")

	if a.notifier != nil {
		a.notifier.CheckClaudeQuota("gemini_usage", snapshot.OverallUsedPct)
	}

	a.logger.Info("Gemini poll complete",
		"tier", snapshot.Tier, "models", len(snapshot.Models),
		"overallUsedPct", snapshot.OverallUsedPct)
}
