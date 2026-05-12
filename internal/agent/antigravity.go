package agent

import (
	"context"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// pollAntigravity fetches Antigravity quota data with backoff management.
func (a *PollingAgent) pollAntigravity(ctx context.Context) {
	// Backoff: if we've failed too many times, skip this tick
	a.mu.Lock()
	if a.failCount >= a.maxFails {
		a.mu.Unlock()
		a.logger.Debug("Auto-capture paused (backoff)", "consecutiveFailures", a.failCount)
		// Every 3rd skip, try once to see if LS recovered
		a.mu.Lock()
		a.failCount++ // increment so we try again after maxFails*2
		if a.failCount >= a.maxFails*2 {
			a.failCount = 0 // reset to retry
			a.logger.Info("Auto-capture: retrying after backoff")
		}
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	// Attempt to fetch quotas
	resp, err := a.client.FetchQuotas(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return // shutdown, not a failure
		}
		a.mu.Lock()
		a.failCount++
		a.lastPollTime = time.Now().UTC()
		a.lastPollOK = false
		count := a.failCount
		a.mu.Unlock()

		a.logger.Warn("Auto-capture failed",
			"error", err,
			"consecutiveFailures", count,
		)

		a.store.LogError("server", "snap_failed", "", map[string]interface{}{
			"error":  err.Error(),
			"method": "auto",
		})
		return
	}

	// Success — reset backoff
	a.mu.Lock()
	a.failCount = 0
	a.lastPollTime = time.Now().UTC()
	a.lastPollOK = true
	a.mu.Unlock()

	snap := resp.ToSnapshot(time.Now().UTC())

	// Tag provenance: auto-capture via server polling
	snap.CaptureMethod = "auto"
	snap.CaptureSource = "server"
	snap.SourceID = "antigravity"

	accountID, err := a.store.GetOrCreateAccount(snap.Email, snap.PlanName, "antigravity")
	if err != nil {
		a.logger.Error("Auto-capture: account error", "error", err, "email", snap.Email)
		return
	}
	snap.AccountID = accountID

	snapID, err := a.store.InsertSnapshot(snap)
	if err != nil {
		a.logger.Error("Auto-capture: insert error", "error", err)
		return
	}

	// Log successful snap
	a.store.LogInfoSnap("server", "snap", snap.Email, snapID, map[string]interface{}{
		"plan": snap.PlanName, "method": "auto", "source": "server",
	})

	// Update data source bookkeeping
	a.store.UpdateSourceCapture("antigravity")

	// Auto-link subscription if needed
	a.autoLink(*snap, accountID)

	// Feed tracker for cycle detection
	if a.tracker != nil {
		if err := a.tracker.Process(snap, accountID); err != nil {
			a.logger.Warn("Tracker error", "error", err)
		}
	}

	// Check notification thresholds for each model
	if a.notifier != nil {
		for _, m := range snap.Models {
			a.notifier.CheckQuota(m.ModelID, m.RemainingPercent)
		}
	}

	// Feed session manager with model remaining fractions
	if a.antigravitySM != nil {
		var vals []float64
		for _, m := range snap.Models {
			vals = append(vals, m.RemainingFraction)
		}
		a.antigravitySM.ReportPoll(vals)
	}

	a.logger.Info("Auto-capture complete",
		"email", snap.Email,
		"plan", snap.PlanName,
		"snapshotId", snapID,
	)
}

// autoLink creates a subscription record if one doesn't exist for this account.
func (a *PollingAgent) autoLink(snap client.Snapshot, accountID int64) {
	autoLinkEnabled := a.store.GetConfigBool("auto_link_subs")
	if !autoLinkEnabled {
		return
	}

	existing, _ := a.store.FindSubscriptionByAccountID(accountID)
	if existing != nil {
		return
	}

	autoSub := &store.Subscription{
		Platform:      "Antigravity",
		Category:      "coding",
		Email:         snap.Email,
		PlanName:      snap.PlanName,
		Status:        "active",
		CostCurrency:  "USD",
		BillingCycle:  "monthly",
		LimitPeriod:   "rolling_5h",
		Notes:         "Auto-created from auto-capture. 5h sprint cycle quotas.",
		URL:           "https://antigravity.google",
		StatusPageURL: "https://status.google.com",
		AutoTracked:   true,
		AccountID:     accountID,
	}
	switch {
	case strings.Contains(strings.ToLower(snap.PlanName), "pro+"),
		strings.Contains(strings.ToLower(snap.PlanName), "ultimate"):
		autoSub.CostAmount = 60
	default:
		autoSub.CostAmount = 15
	}

	if _, err := a.store.InsertSubscription(autoSub); err != nil {
		a.logger.Warn("Auto-link subscription failed", "error", err, "email", snap.Email)
	} else {
		a.store.LogInfo("server", "auto_link", snap.Email, map[string]interface{}{
			"platform": "Antigravity", "plan": snap.PlanName,
		})
		a.logger.Info("Auto-linked subscription", "email", snap.Email, "plan", snap.PlanName)
	}
}
