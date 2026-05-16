// Package agent provides background polling for auto-capture.
package agent

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/codex"
	"github.com/bhaskarjha-com/niyantra/internal/notify"
	"github.com/bhaskarjha-com/niyantra/internal/plugin"
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

	// Cursor state
	cursorAuthFails int

	// Gemini state
	geminiAuthFails int

	// Copilot state
	copilotAuthFails int

	// F18: Plugin system
	plugins []*plugin.Plugin

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
// The poll interval is re-read from the store on each iteration,
// so changes via Settings take effect on the next cycle without restart.
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

	for {
		// Re-read interval from store each iteration (F2: live reload)
		interval := a.store.GetConfigInt("poll_interval", 300)
		if interval < 30 {
			interval = 30
		}

		timer := time.NewTimer(time.Duration(interval) * time.Second)
		select {
		case <-timer.C:
			a.poll(ctx)
		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}

// poll performs a single capture cycle across all providers.
func (a *PollingAgent) poll(ctx context.Context) {
	// Check if polling is enabled via config
	if a.pollingCheck != nil && !a.pollingCheck() {
		return
	}

	// Antigravity: primary provider with backoff management
	a.pollAntigravity(ctx)

	// Claude Code bridge: read statusline data if bridge is enabled
	a.pollClaudeBridge()

	// Codex polling: if codex_capture is enabled
	a.pollCodex(ctx)

	// Cursor polling: if cursor_capture is enabled
	a.pollCursor(ctx)

	// Gemini CLI polling: if gemini_capture is enabled
	a.pollGemini(ctx)

	// GitHub Copilot polling: if copilot_capture is enabled
	a.pollCopilot(ctx)

	// F18: External plugin polling
	a.pollPlugins(ctx)

	// Data retention cleanup: delete snapshots older than retention_days
	a.cleanupOldSnapshots()
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
