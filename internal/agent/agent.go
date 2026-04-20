// Package agent provides background polling for auto-capture.
package agent

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/claudebridge"
	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/codex"
	"github.com/bhaskarjha-com/niyantra/internal/notify"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tracker"
)

// PollingAgent polls the Antigravity language server at a configurable interval.
type PollingAgent struct {
	client   *client.Client
	store    *store.Store
	tracker  *tracker.Tracker
	interval time.Duration
	logger   *slog.Logger

	// pollingCheck is called before each tick; return false to skip.
	pollingCheck func() bool

	notifier *notify.Engine

	// Session managers for usage detection
	antigravitySM *tracker.SessionManager
	codexSM       *tracker.SessionManager
	claudeSM      *tracker.SessionManager

	// Codex state
	codexClient    *codex.Client
	codexAuthFails int

	// Backoff state for consecutive failures
	mu           sync.Mutex
	failCount    int
	maxFails     int // pause after this many consecutive failures
	lastPollTime time.Time
	lastPollOK   bool
}

// NewPollingAgent creates a new auto-capture agent.
func NewPollingAgent(c *client.Client, s *store.Store, t *tracker.Tracker, interval time.Duration, logger *slog.Logger) *PollingAgent {
	return &PollingAgent{
		client:   c,
		store:    s,
		tracker:  t,
		interval: interval,
		logger:   logger,
		maxFails: 3,
	}
}

// SetPollingCheck sets the function called before each poll to check if polling is enabled.
func (a *PollingAgent) SetPollingCheck(fn func() bool) {
	a.pollingCheck = fn
}

// SetNotifier sets the notification engine for quota alerts.
func (a *PollingAgent) SetNotifier(n *notify.Engine) {
	a.notifier = n
}

// SetSessionManagers initializes session detection for all providers.
func (a *PollingAgent) SetSessionManagers(idleTimeout time.Duration) {
	a.antigravitySM = tracker.NewSessionManager(a.store, "antigravity", idleTimeout, a.logger)
	a.codexSM = tracker.NewSessionManager(a.store, "codex", idleTimeout, a.logger)
	a.claudeSM = tracker.NewSessionManager(a.store, "claude", idleTimeout, a.logger)
}

// LastPollTime returns the time of the last poll attempt.
func (a *PollingAgent) LastPollTime() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastPollTime
}

// LastPollOK returns whether the last poll was successful.
func (a *PollingAgent) LastPollOK() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastPollOK
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (a *PollingAgent) Run(ctx context.Context) error {
	a.logger.Info("Auto-capture agent started", "interval", a.interval)
	defer func() {
		// Close any active sessions on shutdown
		if a.antigravitySM != nil {
			a.antigravitySM.Close()
		}
		if a.codexSM != nil {
			a.codexSM.Close()
		}
		if a.claudeSM != nil {
			a.claudeSM.Close()
		}
		a.logger.Info("Auto-capture agent stopped")
	}()

	// Poll immediately on start
	a.poll(ctx)

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.poll(ctx)
		case <-ctx.Done():
			return nil
		}
	}
}

// poll performs a single capture cycle.
func (a *PollingAgent) poll(ctx context.Context) {
	// Check if polling is enabled via config
	if a.pollingCheck != nil && !a.pollingCheck() {
		return
	}

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

	accountID, err := a.store.GetOrCreateAccount(snap.Email, snap.PlanName)
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

	// Claude Code bridge: read statusline data if bridge is enabled
	a.pollClaudeBridge()

	// Codex polling: if codex_capture is enabled
	a.pollCodex(ctx)

	// Data retention cleanup: delete snapshots older than retention_days
	a.cleanupOldSnapshots()

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

// pollClaudeBridge reads Claude Code statusline data and stores a snapshot.
// Called alongside each Antigravity poll when the bridge is enabled.
func (a *PollingAgent) pollClaudeBridge() {
	if !a.store.GetConfigBool("claude_bridge") {
		return
	}

	if !claudebridge.IsFresh(claudebridge.DefaultStaleness) {
		return
	}

	rl, err := claudebridge.ReadData()
	if err != nil {
		a.logger.Debug("Claude bridge read error", "error", err)
		return
	}
	if !claudebridge.IsValid(rl) {
		return
	}

	// Build snapshot values
	var fiveHourPct float64
	var sevenDayPct *float64
	var fiveReset, sevenReset *time.Time

	if rl.FiveHour != nil {
		fiveHourPct = rl.FiveHour.UsedPercentage
		if rl.FiveHour.ResetsAt > 0 {
			t := time.Unix(rl.FiveHour.ResetsAt, 0).UTC()
			fiveReset = &t
		}
	}
	if rl.SevenDay != nil {
		v := rl.SevenDay.UsedPercentage
		sevenDayPct = &v
		if rl.SevenDay.ResetsAt > 0 {
			t := time.Unix(rl.SevenDay.ResetsAt, 0).UTC()
			sevenReset = &t
		}
	}

	if _, err := a.store.InsertClaudeSnapshot(fiveHourPct, sevenDayPct, fiveReset, sevenReset, "statusline"); err != nil {
		a.logger.Error("Failed to store Claude Code snapshot", "error", err)
		return
	}

	// Update data source bookkeeping
	a.store.UpdateSourceCapture("claude_code")

	// Check Claude notification thresholds
	if a.notifier != nil {
		a.notifier.CheckClaudeQuota("five_hour", fiveHourPct)
		if sevenDayPct != nil {
			a.notifier.CheckClaudeQuota("seven_day", *sevenDayPct)
		}
	}

	// Ensure bridge is still healthy
	claudebridge.EnsureBridge(a.logger)

	// Feed Claude session manager
	if a.claudeSM != nil {
		vals := []float64{fiveHourPct}
		if sevenDayPct != nil {
			vals = append(vals, *sevenDayPct)
		}
		a.claudeSM.ReportPoll(vals)
	}

	a.logger.Debug("Claude Code bridge snapshot stored",
		"five_hour_pct", fiveHourPct,
		"seven_day_pct", sevenDayPct)
}

// cleanupOldSnapshots enforces the retention_days config by deleting old data.
func (a *PollingAgent) cleanupOldSnapshots() {
	retentionDays := a.store.GetConfigInt("retention_days", 365)
	if retentionDays <= 0 {
		return // disabled
	}

	deleted, err := a.store.DeleteSnapshotsOlderThan(retentionDays)
	if err != nil {
		a.logger.Warn("Retention cleanup failed", "error", err)
		return
	}
	if deleted > 0 {
		a.logger.Info("Retention cleanup", "deleted", deleted, "retentionDays", retentionDays)
		a.store.LogInfo("server", "retention_cleanup", "", map[string]interface{}{
			"deleted": deleted, "retentionDays": retentionDays,
		})
	}

	// Also clean up expired/dismissed alerts
	a.store.CleanupExpiredAlerts()
}

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
