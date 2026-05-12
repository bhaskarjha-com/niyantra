package web

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/agent"
	"github.com/bhaskarjha-com/niyantra/internal/claudebridge"
	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/notify"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tracker"
)


// Server is the Niyantra HTTP server.
type Server struct {
	logger     *slog.Logger
	store      *store.Store
	client     *client.Client
	tracker    *tracker.Tracker
	notifier   *notify.Engine
	port       int
	auth       string // "user:pass" or ""
	agentMgr   *agent.Manager
	httpServer *http.Server
	Version    string // injected at startup (e.g. "0.12.0")
}

// NewServer creates a new Niyantra web server.
func NewServer(logger *slog.Logger, s *store.Store, c *client.Client, port int, auth string, version string) *Server {
	srv := &Server{
		logger:   logger,
		store:    s,
		client:   c,
		tracker:  newTrackerWithBaseline(s, logger),
		notifier: notify.NewEngine(logger),
		port:     port,
		auth:     auth,
		agentMgr: agent.NewManager(logger),
		Version:  version,
	}

	// Configure notification engine from stored settings
	srv.notifier.Configure(
		s.GetConfigBool("notify_enabled"),
		s.GetConfigFloat("notify_threshold", 10),
	)

	// F9: Wire tracker → notifier reset callback.
	// When tracker detects a model cycle reset, clear the notification guard
	// so the next low-quota event can fire a fresh notification.
	srv.tracker.SetOnReset(srv.notifier.OnReset)

	// F9: Wire notification → system_alert + activity log.
	// When an OS notification fires, also create an in-app alert and log the event.
	srv.notifier.SetOnNotify(func(model string, remainingPct float64) {
		title := fmt.Sprintf("⚠️ %s quota low", model)
		msg := fmt.Sprintf("%.1f%% remaining — consider switching models", remainingPct)
		s.CreateAlert("quota_low_"+model, "warning", title, msg, map[string]interface{}{
			"model":        model,
			"remainingPct": remainingPct,
		})
		s.LogInfo("notify", "quota_alert", "", map[string]interface{}{
			"model":        model,
			"remainingPct": remainingPct,
		})
	})

	// Setup Claude Code bridge if enabled
	if s.GetConfigBool("claude_bridge") {
		if err := claudebridge.SetupBridge(logger); err != nil {
			logger.Warn("Claude Code bridge setup failed", "error", err)
		}
	}

	// Auto-start polling agent if config says so
	if s.GetConfigBool("auto_capture") {
		srv.startPollingAgent()
	}

	return srv
}

// newTrackerWithBaseline creates a tracker and seeds it from the latest DB snapshots (N5).
func newTrackerWithBaseline(s *store.Store, logger *slog.Logger) *tracker.Tracker {
	t := tracker.New(s, logger)
	t.LoadBaseline()
	return t
}

// startPollingAgent creates and starts the auto-capture polling agent.
func (s *Server) startPollingAgent() {
	interval := s.store.GetConfigInt("poll_interval", 300)
	if interval < 30 {
		interval = 30
	}

	ag := agent.NewPollingAgent(s.client, s.store, s.tracker, time.Duration(interval)*time.Second, s.logger)
	ag.SetPollingCheck(func() bool {
		return s.store.GetConfigBool("auto_capture")
	})
	ag.SetNotifier(s.notifier)

	// Initialize session managers with configurable idle timeout
	idleTimeout := time.Duration(s.store.GetConfigInt("session_idle_timeout", 1200)) * time.Second
	ag.SetSessionManagers(idleTimeout)

	s.agentMgr.Start(ag)
	s.logger.Info("Auto-capture started", "interval", interval, "sessionIdleTimeout", idleTimeout)
}

// stopPollingAgent stops the auto-capture polling agent.
func (s *Server) stopPollingAgent() {
	s.agentMgr.Stop()
	s.logger.Info("Auto-capture stopped")
}

// Shutdown gracefully drains in-flight HTTP requests and stops the agent.
func (s *Server) Shutdown() {
	s.agentMgr.Stop()
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Error("HTTP server shutdown error", "error", err)
		}
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// Quota API routes (auto-tracked)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/snap", s.handleSnap)
	mux.HandleFunc("GET /api/history", s.handleHistory)

	// Subscription API routes (manual tracking)
	mux.HandleFunc("GET /api/subscriptions", s.listSubscriptions)
	mux.HandleFunc("POST /api/subscriptions", s.createSubscription)
	mux.HandleFunc("GET /api/subscriptions/{id}", s.getSubscriptionByID)
	mux.HandleFunc("PUT /api/subscriptions/{id}", s.updateSubscriptionByID)
	mux.HandleFunc("DELETE /api/subscriptions/{id}", s.deleteSubscriptionByID)
	mux.HandleFunc("GET /api/overview", s.handleOverview)
	mux.HandleFunc("GET /api/presets", s.handlePresets)
	mux.HandleFunc("GET /api/export/csv", s.handleExportCSV)

	// Config & infrastructure routes (v3)
	mux.HandleFunc("GET /api/config", s.handleConfigGet)
	mux.HandleFunc("PUT /api/config", s.handleConfigPut)
	mux.HandleFunc("GET /api/activity", s.handleActivity)
	mux.HandleFunc("GET /api/mode", s.handleMode)
	mux.HandleFunc("GET /api/usage", s.handleUsage)

	// Phase 9 routes
	mux.HandleFunc("GET /api/claude/status", s.handleClaudeStatus)
	mux.HandleFunc("GET /api/backup", s.handleBackup)
	mux.HandleFunc("POST /api/notify/test", s.handleNotifyTest)

	// Phase 10 routes
	mux.HandleFunc("GET /api/export/json", s.handleExportJSON)
	mux.HandleFunc("GET /api/alerts", s.handleAlerts)
	mux.HandleFunc("POST /api/alerts/dismiss", s.handleDismissAlert)
	mux.HandleFunc("GET /api/advisor", s.handleAdvisor)

	// Phase 11 routes
	mux.HandleFunc("GET /api/codex/status", s.handleCodexStatus)
	mux.HandleFunc("POST /api/codex/snap", s.handleCodexSnap)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/usage-logs", s.handleUsageLogsGet)
	mux.HandleFunc("POST /api/usage-logs", s.handleUsageLogsPost)
	mux.HandleFunc("DELETE /api/usage-logs/{id}", s.handleUsageLogByID)
	mux.HandleFunc("POST /api/import/json", s.handleImportJSON)

	// Phase 13 routes
	mux.HandleFunc("GET /api/config/pricing", s.handleModelPricingGet)
	mux.HandleFunc("PUT /api/config/pricing", s.handleModelPricingPut)

	// Phase 14 routes
	mux.HandleFunc("GET /api/forecast", s.handleForecast)
	mux.HandleFunc("GET /api/cost", s.handleCost)

	// Data management routes
	mux.HandleFunc("GET /api/accounts", s.handleAccounts)
	mux.HandleFunc("GET /api/accounts/{id}", s.handleAccountGet)
	mux.HandleFunc("PATCH /api/accounts/{id}/meta", s.handleAccountMeta)
	mux.HandleFunc("DELETE /api/accounts/{id}", s.handleAccountDelete)
	mux.HandleFunc("DELETE /api/accounts/{id}/snapshots", s.handleAccountClearSnapshots)
	mux.HandleFunc("DELETE /api/snapshots/{id}", s.handleSnapshotByID)
	mux.HandleFunc("PATCH /api/snap/adjust", s.handleSnapAdjust)
	mux.HandleFunc("POST /api/snap/adjust", s.handleSnapAdjust)

	// Static files (embedded in prod, disk in dev)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("web: static fs: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	var handler http.Handler = mux
	handler = s.securityMiddleware(handler)
	if s.auth != "" {
		handler = s.basicAuth(handler)
	}

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}
