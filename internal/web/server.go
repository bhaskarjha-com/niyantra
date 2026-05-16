package web

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/agent"
	"github.com/bhaskarjha-com/niyantra/internal/claude"
	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/mcpserver"
	"github.com/bhaskarjha-com/niyantra/internal/notify"
	"github.com/bhaskarjha-com/niyantra/internal/plugin"
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
	bind       string // bind address (default: "127.0.0.1")
	auth       string // "user:pass" or ""
	agentMgr   *agent.Manager
	httpServer *http.Server
	startTime  time.Time // set in NewServer for /healthz uptime
	Version    string    // injected at startup (e.g. "0.12.0")
}

// NewServer creates a new Niyantra web server.
func NewServer(logger *slog.Logger, s *store.Store, c *client.Client, port int, auth string, version string, bind string) *Server {
	srv := &Server{
		logger:   logger,
		store:    s,
		client:   c,
		tracker:  newTrackerWithBaseline(s, logger),
		notifier: notify.NewEngine(logger),
		port:     port,
		bind:     bind,
		auth:     auth,
		agentMgr:  agent.NewManager(logger),
		startTime: time.Now(),
		Version:   version,
	}

	// Configure notification engine from stored settings
	srv.notifier.Configure(
		s.GetConfigBool("notify_enabled"),
		s.GetConfigFloat("notify_threshold", 10),
	)

	// F11: Configure SMTP email channel from stored settings
	srv.notifier.ConfigureSMTP(srv.loadSMTPConfig())

	// F22: Configure webhook channel from stored settings
	srv.notifier.ConfigureWebhook(srv.loadWebhookConfig())

	// F19: Configure WebPush channel from stored settings
	srv.notifier.ConfigureWebPush(srv.loadWebPushConfig())
	srv.notifier.SetGetSubscriptions(func() []notify.WebPushSubscription {
		storeSubs := srv.store.GetWebPushSubscriptions()
		subs := make([]notify.WebPushSubscription, len(storeSubs))
		for i, ss := range storeSubs {
			subs[i] = notify.WebPushSubscription{
				Endpoint: ss.Endpoint,
				Keys: notify.WebPushKeys{Auth: ss.KeyAuth, P256dh: ss.KeyP256dh},
			}
		}
		return subs
	})

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
		if err := claude.SetupBridge(logger); err != nil {
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

	// F18: Discover and load plugins
	pluginsDir := plugin.DefaultPluginsDir()
	plugins, errs := plugin.Discover(pluginsDir)
	for _, e := range errs {
		s.logger.Warn("Plugin discovery error", "error", e)
	}
	// Load enabled state and config for each plugin from SQLite
	for _, p := range plugins {
		p.Enabled = s.store.GetConfigBool("plugin_" + p.Manifest.ID + "_enabled")
		for key := range p.Manifest.Config {
			val := s.store.GetConfig("plugin_" + p.Manifest.ID + "_" + key)
			if val != "" {
				p.Config[key] = val
			}
		}
		if p.Enabled {
			// Register data source if not exists
			s.registerPluginDataSource(p)
		}
	}
	ag.SetPlugins(plugins)
	if len(plugins) > 0 {
		enabled := 0
		for _, p := range plugins {
			if p.Enabled {
				enabled++
			}
		}
		s.logger.Info("Plugins discovered", "total", len(plugins), "enabled", enabled, "dir", pluginsDir)
	}

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

	// Operational endpoints (no auth required — registered on inner mux)
	mux.HandleFunc("GET /healthz", s.handleHealthz)

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
	mux.HandleFunc("GET /api/claude/usage", s.handleClaudeUsage)
	mux.HandleFunc("GET /api/backup", s.handleBackup)
	mux.HandleFunc("POST /api/notify/test", s.handleNotifyTest)
	mux.HandleFunc("POST /api/notify/test-email", s.handleNotifyTestEmail)
	mux.HandleFunc("POST /api/notify/test-webhook", s.handleNotifyTestWebhook)
	mux.HandleFunc("POST /api/notify/test-webpush", s.handleNotifyTestWebPush)

	// F19: WebPush routes
	mux.HandleFunc("GET /api/webpush/vapid-key", s.handleWebPushVAPIDKey)
	mux.HandleFunc("POST /api/webpush/subscribe", s.handleWebPushSubscribe)
	mux.HandleFunc("DELETE /api/webpush/subscribe", s.handleWebPushUnsubscribe)
	mux.HandleFunc("GET /api/webpush/status", s.handleWebPushStatus)

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

	// Phase 14 routes: Cursor provider
	mux.HandleFunc("GET /api/cursor/status", s.handleCursorStatus)
	mux.HandleFunc("POST /api/cursor/snap", s.handleCursorSnap)

	// Phase 14 routes: Gemini CLI provider (F15b)
	mux.HandleFunc("GET /api/gemini/status", s.handleGeminiStatus)
	mux.HandleFunc("POST /api/gemini/snap", s.handleGeminiSnap)

	// Phase 15 routes: GitHub Copilot provider (F15c)
	mux.HandleFunc("GET /api/copilot/status", s.handleCopilotStatus)
	mux.HandleFunc("POST /api/copilot/snap", s.handleCopilotSnap)

	// Phase 13 routes
	mux.HandleFunc("GET /api/config/pricing", s.handleModelPricingGet)
	mux.HandleFunc("PUT /api/config/pricing", s.handleModelPricingPut)

	// Phase 14 routes
	mux.HandleFunc("GET /api/forecast", s.handleForecast)
	mux.HandleFunc("GET /api/cost", s.handleCost)
	mux.HandleFunc("GET /api/history/heatmap", s.handleHeatmap)

	// Phase 15 routes: Token Usage Analytics (F13)
	mux.HandleFunc("GET /api/token-usage", s.handleTokenUsage)

	// Phase 15 routes: Git Commit Correlation (F16)
	mux.HandleFunc("GET /api/git-costs", s.handleGitCosts)

	// Phase 15 routes: Streamable HTTP MCP (F14)
	// The MCP SDK handler manages its own Origin/Content-Type verification,
	// session management, and SSE streaming. We mount it directly so the
	// security middleware doesn't interfere with the MCP protocol.
	mcpSrv := mcpserver.New(s.store, s.tracker, s.logger, s.Version)
	mux.Handle("/mcp", mcpSrv.HTTPHandler())

	// Data management routes
	mux.HandleFunc("GET /api/accounts", s.handleAccounts)
	mux.HandleFunc("GET /api/accounts/{id}", s.handleAccountGet)
	mux.HandleFunc("PATCH /api/accounts/{id}/meta", s.handleAccountMeta)
	mux.HandleFunc("DELETE /api/accounts/{id}", s.handleAccountDelete)
	mux.HandleFunc("DELETE /api/accounts/{id}/snapshots", s.handleAccountClearSnapshots)
	mux.HandleFunc("DELETE /api/snapshots/{id}", s.handleSnapshotByID)
	mux.HandleFunc("PATCH /api/snap/adjust", s.handleSnapAdjust)
	mux.HandleFunc("POST /api/snap/adjust", s.handleSnapAdjust)

	// Phase 16 routes: Plugin System (F18)
	mux.HandleFunc("GET /api/plugins", s.handlePlugins)
	mux.HandleFunc("GET /api/plugins/{id}/status", s.handlePluginStatus)
	mux.HandleFunc("POST /api/plugins/{id}/run", s.handlePluginRun)
	mux.HandleFunc("PUT /api/plugins/{id}/config", s.handlePluginConfig)

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
		Addr:              fmt.Sprintf("%s:%d", s.bind, s.port),
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}
