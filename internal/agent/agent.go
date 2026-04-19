// Package agent provides background polling for auto-capture.
package agent

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/claudebridge"
	"github.com/bhaskarjha-com/niyantra/internal/client"
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

	// Backoff state for consecutive failures
	mu            sync.Mutex
	failCount     int
	maxFails      int           // pause after this many consecutive failures
	lastPollTime  time.Time
	lastPollOK    bool
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
	defer a.logger.Info("Auto-capture agent stopped")

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

	// Claude Code bridge: read statusline data if bridge is enabled
	a.pollClaudeBridge()

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
		Platform:     "Antigravity",
		Category:     "coding",
		Email:        snap.Email,
		PlanName:     snap.PlanName,
		Status:       "active",
		CostCurrency: "USD",
		BillingCycle: "monthly",
		LimitPeriod:  "rolling_5h",
		Notes:        "Auto-created from auto-capture. 5h sprint cycle quotas.",
		URL:          "https://windsurf.com",
		StatusPageURL: "https://status.google.com",
		AutoTracked:  true,
		AccountID:    accountID,
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

	a.logger.Debug("Claude Code bridge snapshot stored",
		"five_hour_pct", fiveHourPct,
		"seven_day_pct", sevenDayPct)
}
