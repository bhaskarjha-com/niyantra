package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/advisor"
	"github.com/bhaskarjha-com/niyantra/internal/agent"
	"github.com/bhaskarjha-com/niyantra/internal/claudebridge"
	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/codex"
	"github.com/bhaskarjha-com/niyantra/internal/costtrack"
	"github.com/bhaskarjha-com/niyantra/internal/forecast"
	"github.com/bhaskarjha-com/niyantra/internal/notify"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
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

// handleStatus returns readiness for all accounts.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snapshots, err := s.store.LatestPerAccount()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	accounts := readiness.Calculate(snapshots, 0.0)

	// F1: Enrich readiness results with account notes/tags/pinned_group/creditRenewalDay
	for i := range accounts {
		notes, tags, pinnedGroup, creditRenewalDay, err := s.store.AccountMeta(accounts[i].AccountID)
		if err == nil {
			accounts[i].Notes = notes
			accounts[i].Tags = tags
			accounts[i].PinnedGroup = pinnedGroup
			accounts[i].CreditRenewalDay = creditRenewalDay
		}
	}

	result := map[string]interface{}{
		"accounts":      accounts,
		"snapshotCount": s.store.SnapshotCount(),
		"accountCount":  s.store.AccountCount(),
	}

	// C4: Include Codex snapshot if available (for homepage grid)
	codexSnap, _ := s.store.LatestCodexSnapshot()
	if codexSnap != nil {
		result["codexSnapshot"] = codexSnap
	}

	// C4: Include Claude snapshot if available
	claudeSnap, _ := s.store.LatestClaudeSnapshot()
	if claudeSnap != nil {
		result["claudeSnapshot"] = claudeSnap
	}

	// F7: Compute per-account forecasts using sliding-window rates
	forecastsByAccount := s.computeAccountForecasts(snapshots)
	if forecastsByAccount != nil {
		result["forecasts"] = forecastsByAccount
	}

	// F8: Compute per-account estimated costs using forecast rates + model pricing
	costsByAccount := s.computeAccountCosts(snapshots, forecastsByAccount)
	if costsByAccount != nil {
		result["estimatedCosts"] = costsByAccount
	}

	writeJSON(w, result)
}

// handleSnap triggers a snapshot capture.
func (s *Server) handleSnap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp, err := s.client.FetchQuotas(ctx)
	if err != nil {
		s.logger.Error("snap failed", "error", err)
		s.store.LogError("ui", "snap_failed", "", map[string]interface{}{
			"error": err.Error(),
		})
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}

	snap := resp.ToSnapshot(time.Now().UTC())

	// Tag provenance: captured via dashboard UI
	snap.CaptureMethod = "manual"
	snap.CaptureSource = "ui"
	snap.SourceID = "antigravity"

	accountID, err := s.store.GetOrCreateAccount(snap.Email, snap.PlanName)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	snap.AccountID = accountID

	snapID, err := s.store.InsertSnapshot(snap)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Log successful snap
	s.store.LogInfoSnap("ui", "snap", snap.Email, snapID, map[string]interface{}{
		"plan": snap.PlanName, "method": "manual", "source": "ui",
	})

	// Update data source bookkeeping
	s.store.UpdateSourceCapture("antigravity")

	// Auto-link: create a subscription record if one doesn't exist for this account
	// Respects auto_link_subs config toggle (S2: was previously ignoring it)
	if s.store.GetConfig("auto_link_subs") != "false" {
		existing, _ := s.store.FindSubscriptionByAccountID(accountID)
		if existing == nil {
			autoSub := &store.Subscription{
				Platform:      "Antigravity",
				Category:      "coding",
				Email:         snap.Email,
				PlanName:      snap.PlanName,
				Status:        "active",
				CostCurrency:  "USD",
				BillingCycle:  "monthly",
				LimitPeriod:   "rolling_5h",
				Notes:         "Auto-created from quota snapshot. 5h sprint cycle quotas.",
				URL:           "https://antigravity.google",
				StatusPageURL: "https://status.google.com",
				AutoTracked:   true,
				AccountID:     accountID,
			}
			// Set cost based on plan name heuristic
			switch {
			case strings.Contains(strings.ToLower(snap.PlanName), "pro+"),
				strings.Contains(strings.ToLower(snap.PlanName), "ultimate"):
				autoSub.CostAmount = 60
			default:
				autoSub.CostAmount = 15
			}
			if _, err := s.store.InsertSubscription(autoSub); err != nil {
				s.logger.Warn("auto-link subscription failed", "error", err, "email", snap.Email)
			} else {
				s.logger.Info("auto-linked subscription", "email", snap.Email, "plan", snap.PlanName)
			}
		}
	}

	// Feed tracker for cycle intelligence (also works for manual snaps)
	if s.tracker != nil {
		if err := s.tracker.Process(snap, accountID); err != nil {
			s.logger.Warn("tracker error on manual snap", "error", err)
		}
	}

	// Return updated accounts
	snapshots, _ := s.store.LatestPerAccount()
	accounts := readiness.Calculate(snapshots, 0.0)

	writeJSON(w, map[string]interface{}{
		"message":       "snapshot captured",
		"email":         snap.Email,
		"planName":      snap.PlanName,
		"snapshotId":    snapID,
		"accountId":     accountID,
		"accounts":      accounts,
		"accountCount":  s.store.AccountCount(),
		"snapshotCount": s.store.SnapshotCount(),
	})
}

// handleHistory returns snapshot history.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	var accountID int64
	if v := r.URL.Query().Get("account"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			accountID = id
		}
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	snapshots, err := s.store.History(accountID, limit)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Convert snapshots to API format with groups
	type snapResponse struct {
		ID            int64                 `json:"id"`
		AccountID     int64                 `json:"accountId"`
		Email         string                `json:"email"`
		CapturedAt    time.Time             `json:"capturedAt"`
		PlanName      string                `json:"planName"`
		Groups        []client.GroupedQuota `json:"groups"`
		CaptureMethod string                `json:"captureMethod"`
		CaptureSource string                `json:"captureSource"`
	}

	var items []snapResponse
	for _, s := range snapshots {
		items = append(items, snapResponse{
			ID:            s.ID,
			AccountID:     s.AccountID,
			Email:         s.Email,
			CapturedAt:    s.CapturedAt,
			PlanName:      s.PlanName,
			Groups:        client.GroupModels(s.Models),
			CaptureMethod: s.CaptureMethod,
			CaptureSource: s.CaptureSource,
		})
	}

	writeJSON(w, map[string]interface{}{
		"snapshots": items,
	})
}

// handleConfigGet returns server configuration entries.
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	entries, err := s.store.AllConfig(category)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"config": entries})
}

// handleConfigPut updates a single configuration key.
func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		jsonError(w, "key is required", http.StatusBadRequest)
		return
	}

	oldVal, err := s.store.SetConfig(req.Key, req.Value)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the config change
	s.store.LogInfo("ui", "config_change", "", map[string]interface{}{
		"key": req.Key, "from": oldVal, "to": req.Value,
	})

	// React to auto_capture toggle
	if req.Key == "auto_capture" {
		if req.Value == "true" {
			s.startPollingAgent()
		} else {
			s.stopPollingAgent()
		}
	}

	// F2: poll_interval changes are picked up automatically by the agent
	// on the next tick — no restart needed.

	// React to bridge/notification config changes
	s.onConfigChanged(req.Key, req.Value)

	entries, _ := s.store.AllConfig("")
	writeJSON(w, map[string]interface{}{"config": entries})
}

// handleActivity returns recent activity log entries.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	eventType := r.URL.Query().Get("type")

	entries, err := s.store.RecentActivity(limit, eventType)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []*store.ActivityEntry{}
	}

	writeJSON(w, map[string]interface{}{"entries": entries})
}

// handleMode returns lightweight capture mode status for the header badge.
func (s *Server) handleMode(w http.ResponseWriter, r *http.Request) {
	autoCapture := s.store.GetConfigBool("auto_capture")
	mode := "manual"
	if autoCapture {
		mode = "auto"
	}

	sources, _ := s.store.AllDataSources()
	if sources == nil {
		sources = []*store.DataSource{}
	}

	result := map[string]interface{}{
		"mode":          mode,
		"autoCapture":   autoCapture,
		"isPolling":     s.agentMgr.IsRunning(),
		"pollInterval":  s.store.GetConfigInt("poll_interval", 300),
		"sources":       sources,
		"schemaVersion": s.store.SchemaVersion(),
	}

	// Add last poll info if agent is running
	if ag := s.agentMgr.Agent(); ag != nil {
		if t := ag.LastPollTime(); !t.IsZero() {
			result["lastPoll"] = t
			result["lastPollOK"] = ag.LastPollOK()
		}
	}

	writeJSON(w, result)
}

// handleUsage returns per-model usage intelligence and budget forecast.
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	var accountID int64
	if v := r.URL.Query().Get("account"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			accountID = id
		}
	}

	result := map[string]interface{}{
		"models":         nil,
		"budgetForecast": nil,
	}

	// Get latest snapshot(s) for the account(s)
	// N3b: When no account filter is specified, aggregate across all accounts
	snapshots, _ := s.store.LatestPerAccount()
	var allModels []interface{}
	for _, snap := range snapshots {
		if accountID > 0 && snap.AccountID != accountID {
			continue
		}

		if s.tracker != nil {
			summaries, err := s.tracker.AllUsageSummaries(snap, snap.AccountID)
			if err != nil {
				s.logger.Warn("usage summary error", "error", err)
			}
			if summaries != nil {
				if accountID > 0 {
					// Single account filter — return directly
					result["models"] = summaries
					break
				}
				// Aggregate across all accounts
				for _, s := range summaries {
					allModels = append(allModels, s)
				}
			}
		}
	}
	if accountID == 0 && len(allModels) > 0 {
		result["models"] = allModels
	}

	// Budget forecast
	forecast := tracker.ComputeBudgetForecast(s.store)
	if forecast != nil {
		result["budgetForecast"] = forecast
	}

	writeJSON(w, result)
}

// basicAuth wraps a handler with HTTP basic authentication.
func (s *Server) basicAuth(next http.Handler) http.Handler {
	parts := strings.SplitN(s.auth, ":", 2)
	if len(parts) != 2 {
		return next
	}
	user, pass := parts[0], parts[1]

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Niyantra"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityMiddleware enforces CORS and Content-Type policies.
func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS: only allow localhost origin matching our port
		allowedOrigin := fmt.Sprintf("http://localhost:%d", s.port)
		allowedOrigin2 := fmt.Sprintf("http://127.0.0.1:%d", s.port)
		origin := r.Header.Get("Origin")
		if origin == allowedOrigin || origin == allowedOrigin2 {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Enforce Content-Type: application/json on mutation endpoints
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				ct := r.Header.Get("Content-Type")
				// Allow empty content-type for DELETE and requests with no body
				if ct != "" && !strings.HasPrefix(ct, "application/json") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnsupportedMediaType)
					json.NewEncoder(w).Encode(map[string]string{"error": "Content-Type must be application/json"})
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ── Phase 9 Handlers ─────────────────────────────────────────────

// handleClaudeStatus returns the current Claude Code rate limit data.
func (s *Server) handleClaudeStatus(w http.ResponseWriter, r *http.Request) {
	bridgeEnabled := s.store.GetConfigBool("claude_bridge")
	installed := claudebridge.IsClaudeCodeInstalled()
	fresh := claudebridge.IsFresh(claudebridge.DefaultStaleness)

	result := map[string]interface{}{
		"installed":     installed,
		"bridgeEnabled": bridgeEnabled,
		"bridgeFresh":   fresh,
		"supported":     notify.IsSupported(),
	}

	// Get latest snapshot from DB
	snap, err := s.store.LatestClaudeSnapshot()
	if err != nil {
		s.logger.Error("Failed to get Claude snapshot", "error", err)
	}
	if snap != nil {
		snapMap := map[string]interface{}{
			"fiveHourPct": snap.FiveHourPct,
			"capturedAt":  snap.CapturedAt.Format(time.RFC3339),
			"source":      snap.Source,
		}
		if snap.SevenDayPct != nil {
			snapMap["sevenDayPct"] = *snap.SevenDayPct
		}
		if snap.FiveHourReset != nil {
			snapMap["fiveHourReset"] = snap.FiveHourReset.Format(time.RFC3339)
		}
		if snap.SevenDayReset != nil {
			snapMap["sevenDayReset"] = snap.SevenDayReset.Format(time.RFC3339)
		}
		result["snapshot"] = snapMap
	}

	writeJSON(w, result)
}

// handleBackup serves a consistent database backup as a download.
// Uses VACUUM INTO for WAL-safe snapshot instead of raw file copy.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	// Create temp file for VACUUM INTO
	backupPath := s.store.Path() + ".backup-" + time.Now().Format("20060102-150405")
	if err := s.store.VacuumInto(backupPath); err != nil {
		s.logger.Error("Backup VACUUM INTO failed", "error", err)
		jsonError(w, "backup failed", http.StatusInternalServerError)
		return
	}
	defer os.Remove(backupPath)

	f, err := os.Open(backupPath)
	if err != nil {
		jsonError(w, "cannot open backup", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		jsonError(w, "cannot stat backup", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("niyantra-%s.db", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	io.Copy(w, f)
}

// handleNotifyTest sends a test notification.
func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	if !notify.IsSupported() {
		jsonError(w, "notifications not supported on this platform", http.StatusBadRequest)
		return
	}

	if err := s.notifier.SendTest(); err != nil {
		jsonError(w, fmt.Sprintf("notification failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "sent"})
}

// onConfigChanged is called after a config key is updated via the API.
// Handles side effects for bridge, notification, and data source settings.
func (s *Server) onConfigChanged(key, value string) {
	switch key {
	case "claude_bridge":
		if value == "true" {
			if err := claudebridge.SetupBridge(s.logger); err != nil {
				s.logger.Warn("Claude Code bridge setup failed", "error", err)
			}
		} else {
			if err := claudebridge.DisableBridge(s.logger); err != nil {
				s.logger.Warn("Claude Code bridge disable failed", "error", err)
			}
		}
		// S7: Sync data_sources.claude_code.enabled to match config
		s.store.SetSourceEnabled("claude_code", value == "true")
	case "codex_capture":
		// S7: Sync data_sources.codex.enabled to match config
		s.store.SetSourceEnabled("codex", value == "true")
	case "notify_enabled", "notify_threshold":
		s.notifier.Configure(
			s.store.GetConfigBool("notify_enabled"),
			s.store.GetConfigFloat("notify_threshold", 10),
		)
	}
}

// ── Phase 10 Handlers ────────────────────────────────────────────

// handleExportJSON exports all data as a JSON file for full portability.
func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	export := map[string]interface{}{
		"version":         "1.0",
		"exportedAt":      time.Now().UTC().Format(time.RFC3339),
		"niyantraVersion": s.Version,
	}

	// Accounts
	accounts, _ := s.store.AllAccounts()
	if accounts == nil {
		accounts = []*store.Account{}
	}
	export["accounts"] = accounts

	// Subscriptions
	subs, _ := s.store.ListSubscriptions("", "")
	if subs == nil {
		subs = []*store.Subscription{}
	}
	export["subscriptions"] = subs

	// Recent snapshots (last 1000)
	snapshots, _ := s.store.History(0, 1000)
	export["snapshots"] = snapshots

	// Claude snapshots (last 500)
	claudeSnaps, _ := s.store.ClaudeSnapshotHistory(500)
	export["claudeSnapshots"] = claudeSnaps

	// Config
	config, _ := s.store.AllConfig("")
	export["config"] = config

	// Activity log (last 500)
	activity, _ := s.store.RecentActivity(500, "")
	export["activityLog"] = activity

	// Log the export event
	s.store.LogInfo("ui", "export", "", map[string]interface{}{
		"format": "json",
	})

	filename := fmt.Sprintf("niyantra-export-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	json.NewEncoder(w).Encode(export)
}

// handleAlerts returns active system alerts.
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := s.store.ActiveAlerts()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if alerts == nil {
		alerts = []*store.SystemAlert{}
	}

	writeJSON(w, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// handleDismissAlert dismisses a system alert by ID.
func (s *Server) handleDismissAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.ID <= 0 {
		jsonError(w, "alert ID required", http.StatusBadRequest)
		return
	}

	if err := s.store.DismissAlert(req.ID); err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"message": "dismissed"})
}

// handleAdvisor returns account switching recommendation.
func (s *Server) handleAdvisor(w http.ResponseWriter, r *http.Request) {
	snapshots, err := s.store.LatestPerAccount()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Build per-account usage summaries for burn rate intelligence
	summariesByAccount := make(map[int64][]*tracker.UsageSummary)
	if s.tracker != nil {
		for _, snap := range snapshots {
			summaries, err := s.tracker.AllUsageSummaries(snap, snap.AccountID)
			if err == nil && len(summaries) > 0 {
				summariesByAccount[snap.AccountID] = summaries
			}
		}
	}

	rec := advisor.Recommend(snapshots, summariesByAccount)
	writeJSON(w, rec)
}

// ── Phase 11 Handlers ────────────────────────────────────────────

// handleCodexStatus returns Codex detection state and latest snapshot.
func (s *Server) handleCodexStatus(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"installed":      false,
		"captureEnabled": s.store.GetConfigBool("codex_capture"),
	}

	// Detect credentials
	creds, err := codex.DetectCredentials(s.logger)
	if err == nil && creds != nil {
		result["installed"] = true
		result["accountId"] = creds.AccountID
		result["email"] = creds.Email
		result["name"] = creds.Name
		result["planExpiry"] = nil
		if !creds.ExpiresAt.IsZero() {
			result["tokenExpiry"] = creds.ExpiresAt.Format(time.RFC3339)
			result["tokenExpiresIn"] = creds.ExpiresIn.Round(time.Minute).String()
			result["tokenExpired"] = creds.IsExpired()
		}
	}

	// Latest snapshot
	snap, err := s.store.LatestCodexSnapshot()
	if err != nil {
		s.logger.Error("Failed to get Codex snapshot", "error", err)
	}
	if snap != nil {
		result["snapshot"] = snap
	}

	writeJSON(w, result)
}

// handleCodexSnap triggers a manual Codex usage snapshot.
func (s *Server) handleCodexSnap(w http.ResponseWriter, r *http.Request) {
	creds, err := codex.DetectCredentials(s.logger)
	if err != nil {
		jsonError(w, fmt.Sprintf("Codex not detected: %v", err), http.StatusBadRequest)
		return
	}

	// Refresh token if expired
	if creds.IsExpired() && creds.RefreshToken != "" {
		s.logger.Info("Codex token expired, refreshing for manual snap")
		newTokens, refreshErr := codex.RefreshToken(r.Context(), creds.RefreshToken)
		if refreshErr != nil {
			jsonError(w, fmt.Sprintf("Token refresh failed: %v", refreshErr), http.StatusBadGateway)
			return
		}
		if err := codex.WriteCredentials(newTokens.AccessToken, newTokens.RefreshToken, newTokens.IDToken); err != nil {
			s.logger.Error("Failed to save refreshed Codex tokens", "error", err)
		}
		creds.AccessToken = newTokens.AccessToken
	}

	// Fetch usage
	client := codex.NewClient(creds.AccessToken, creds.AccountID, s.logger)
	usage, err := client.FetchUsage(r.Context())
	if err != nil {
		jsonError(w, fmt.Sprintf("Codex API error: %v", err), http.StatusBadGateway)
		return
	}

	// Build and store snapshot
	snap := &store.CodexSnapshot{
		AccountID:      creds.AccountID,
		Email:          creds.Email,
		FiveHourPct:    0,
		PlanType:       usage.PlanType,
		CreditsBalance: usage.CreditsBalance,
		CaptureMethod:  "manual",
		CaptureSource:  "ui",
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

	snapID, err := s.store.InsertCodexSnapshot(snap)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Update data source bookkeeping
	s.store.UpdateSourceCapture("codex")

	// Log the snap
	s.store.LogInfo("ui", "codex_snap", creds.AccountID, map[string]interface{}{
		"plan": usage.PlanType, "method": "manual",
	})

	writeJSON(w, map[string]interface{}{
		"message":    "Codex snapshot captured",
		"snapshotId": snapID,
		"plan":       usage.PlanType,
		"quotas":     usage.Quotas,
	})
}

// handleSessions returns recent usage sessions.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	sessions, err := s.store.RecentSessions(provider, limit)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []*store.UsageSession{}
	}

	writeJSON(w, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// handleUsageLogsGet returns usage logs for a subscription.
func (s *Server) handleUsageLogsGet(w http.ResponseWriter, r *http.Request) {
	subIDStr := r.URL.Query().Get("subscriptionId")
	if subIDStr == "" {
		jsonError(w, "subscriptionId required", http.StatusBadRequest)
		return
	}
	subID, err := strconv.ParseInt(subIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid subscriptionId", http.StatusBadRequest)
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	logs, err := s.store.UsageLogsForSubscription(subID, limit)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []*store.UsageLog{}
	}

	summary, _ := s.store.UsageLogSummaryFor(subID)

	writeJSON(w, map[string]interface{}{
		"logs":    logs,
		"summary": summary,
	})
}

// handleUsageLogsPost creates a new usage log entry.
func (s *Server) handleUsageLogsPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SubscriptionID int64   `json:"subscriptionId"`
		UsageAmount    float64 `json:"usageAmount"`
		UsageUnit      string  `json:"usageUnit"`
		Notes          string  `json:"notes"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.SubscriptionID <= 0 || req.UsageAmount <= 0 || req.UsageUnit == "" {
		jsonError(w, "subscriptionId, usageAmount, and usageUnit are required", http.StatusBadRequest)
		return
	}

	id, err := s.store.InsertUsageLog(req.SubscriptionID, req.UsageAmount, req.UsageUnit, req.Notes)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"message": "usage logged",
		"id":      id,
	})
}

// handleUsageLogByID handles DELETE for a specific usage log.
func (s *Server) handleUsageLogByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from Go 1.22+ path parameter
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, "invalid usage log ID", http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteUsageLog(id); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"message": "deleted"})
}

// handleImportJSON handles JSON data import with merge strategy.
func (s *Server) handleImportJSON(w http.ResponseWriter, r *http.Request) {
	// Read request body (limit to 50MB)
	body, err := io.ReadAll(io.LimitReader(r.Body, 50<<20))
	if err != nil {
		jsonError(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		jsonError(w, "empty request body", http.StatusBadRequest)
		return
	}

	result, err := s.store.ImportJSON(body)
	if err != nil {
		jsonError(w, fmt.Sprintf("import failed: %v", err), http.StatusBadRequest)
		return
	}

	// Log the import
	s.store.LogInfo("ui", "import", "", map[string]interface{}{
		"accountsCreated":   result.AccountsCreated,
		"accountsSkipped":   result.AccountsSkipped,
		"subsCreated":       result.SubsCreated,
		"subsSkipped":       result.SubsSkipped,
		"snapshotsImported": result.SnapshotsImported,
		"snapshotsDuped":    result.SnapshotsDuped,
		"errors":            len(result.Errors),
	})

	writeJSON(w, result)
}

// ── Data Management Handlers ─────────────────────────────────────

// handleAccounts returns all tracked accounts.
func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.AllAccounts()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if accounts == nil {
		accounts = []*store.Account{}
	}

	writeJSON(w, map[string]interface{}{"accounts": accounts})
}

// handleAccountByID handles DELETE /api/accounts/:id, DELETE /api/accounts/:id/snapshots,
// handleAccountGet returns a single account by ID.
// Currently not needed by the frontend, reserved for future use.
func (s *Server) handleAccountGet(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || accountID <= 0 {
		jsonError(w, "invalid account ID", http.StatusBadRequest)
		return
	}

	accounts, err := s.store.AllAccounts()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	for _, a := range accounts {
		if a.ID == accountID {
			writeJSON(w, a)
			return
		}
	}
	jsonError(w, "account not found", http.StatusNotFound)
}

// handleAccountMeta updates account notes, tags, pinned group, and credit renewal day.
// F1: PATCH /api/accounts/{id}/meta
func (s *Server) handleAccountMeta(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || accountID <= 0 {
		jsonError(w, "invalid account ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Notes            *string `json:"notes"`
		Tags             *string `json:"tags"`
		PinnedGroup      *string `json:"pinnedGroup"`
		CreditRenewalDay *int    `json:"creditRenewalDay"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Read current values to preserve unchanged fields
	currentNotes, currentTags, currentPinned, currentRenewalDay, err := s.store.AccountMeta(accountID)
	if err != nil {
		jsonError(w, "account not found", http.StatusNotFound)
		return
	}

	notes := currentNotes
	tags := currentTags
	pinnedGroup := currentPinned
	creditRenewalDay := currentRenewalDay
	if req.Notes != nil {
		notes = *req.Notes
	}
	if req.Tags != nil {
		tags = *req.Tags
	}
	if req.PinnedGroup != nil {
		pinnedGroup = *req.PinnedGroup
	}
	if req.CreditRenewalDay != nil {
		day := *req.CreditRenewalDay
		if day < 0 || day > 31 {
			jsonError(w, "creditRenewalDay must be 0-31", http.StatusBadRequest)
			return
		}
		creditRenewalDay = day
	}

	if err := s.store.UpdateAccountMeta(accountID, notes, tags, pinnedGroup, creditRenewalDay); err != nil {
		jsonError(w, "update failed", http.StatusInternalServerError)
		return
	}

	s.store.LogInfo("ui", "account_meta_update", "", map[string]interface{}{
		"accountId": accountID, "notes": notes, "tags": tags, "pinnedGroup": pinnedGroup, "creditRenewalDay": creditRenewalDay,
	})

	writeJSON(w, map[string]interface{}{
		"message":          "account meta updated",
		"notes":            notes,
		"tags":             tags,
		"pinnedGroup":      pinnedGroup,
		"creditRenewalDay": creditRenewalDay,
	})
}

// handleAccountDelete performs a full cascade delete of an account and all its data.
func (s *Server) handleAccountDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || accountID <= 0 {
		jsonError(w, "invalid account ID", http.StatusBadRequest)
		return
	}

	deleted, err := s.store.DeleteAccount(accountID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.store.LogInfo("ui", "account_deleted", "", map[string]interface{}{
		"accountId":    accountID,
		"totalDeleted": deleted,
	})

	writeJSON(w, map[string]interface{}{
		"message":      "account deleted",
		"totalDeleted": deleted,
	})
}

// handleAccountClearSnapshots deletes all snapshots for an account without removing the account.
func (s *Server) handleAccountClearSnapshots(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || accountID <= 0 {
		jsonError(w, "invalid account ID", http.StatusBadRequest)
		return
	}

	deleted, err := s.store.DeleteAccountSnapshots(accountID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.store.LogInfo("ui", "snapshots_cleared", "", map[string]interface{}{
		"accountId":        accountID,
		"snapshotsDeleted": deleted,
	})

	writeJSON(w, map[string]interface{}{
		"message":          "snapshots cleared",
		"snapshotsDeleted": deleted,
	})
}

// handleSnapshotByID handles DELETE /api/snapshots/{id}
func (s *Server) handleSnapshotByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, "invalid snapshot ID", http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteSnapshot(id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.store.LogInfo("ui", "snapshot_deleted", "", map[string]interface{}{
		"snapshotId": id,
	})

	writeJSON(w, map[string]string{"message": "snapshot deleted"})
}

// handleSnapAdjust handles PATCH|POST /api/snap/adjust
// Lets users fine-tune model quota percentages on a snapshot.
//
// Request body:
//
//	{
//	  "snapshotId": 42,
//	  "adjustments": [
//	    {"label": "Gemini 3.1 Pro (High)", "remainingPercent": 80},
//	    {"label": "Claude Sonnet 4.6",     "remainingPercent": 45}
//	  ]
//	}
func (s *Server) handleSnapAdjust(w http.ResponseWriter, r *http.Request) {

	var req struct {
		SnapshotID  int64 `json:"snapshotId"`
		Adjustments []struct {
			Label            string  `json:"label"`
			RemainingPercent float64 `json:"remainingPercent"`
		} `json:"adjustments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.SnapshotID <= 0 {
		jsonError(w, "snapshotId is required", http.StatusBadRequest)
		return
	}
	if len(req.Adjustments) == 0 {
		jsonError(w, "at least one adjustment is required", http.StatusBadRequest)
		return
	}

	// Fetch current snapshot to get existing models
	snaps, err := s.store.History(0, 1000)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	var targetSnap *client.Snapshot
	for _, snap := range snaps {
		if snap.ID == req.SnapshotID {
			targetSnap = snap
			break
		}
	}
	if targetSnap == nil {
		jsonError(w, "snapshot not found", http.StatusNotFound)
		return
	}

	// Apply adjustments
	adjustCount := 0
	for i := range targetSnap.Models {
		for _, adj := range req.Adjustments {
			if targetSnap.Models[i].Label == adj.Label {
				pct := adj.RemainingPercent
				if pct < 0 {
					pct = 0
				}
				if pct > 100 {
					pct = 100
				}
				targetSnap.Models[i].RemainingPercent = pct
				targetSnap.Models[i].RemainingFraction = pct / 100
				targetSnap.Models[i].IsExhausted = pct <= 0
				adjustCount++
				break
			}
		}
	}

	if adjustCount == 0 {
		jsonError(w, "no matching models found to adjust", http.StatusBadRequest)
		return
	}

	// Save updated models
	if err := s.store.UpdateSnapshotModels(req.SnapshotID, targetSnap.Models); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.store.LogInfo("ui", "snap_adjusted", targetSnap.Email, map[string]interface{}{
		"snapshotId":  req.SnapshotID,
		"adjustments": adjustCount,
	})

	writeJSON(w, map[string]interface{}{
		"message":     "snapshot adjusted",
		"snapshotId":  req.SnapshotID,
		"adjustments": adjustCount,
		"models":      targetSnap.Models,
	})
}

// ── Phase 13: Model Pricing Config (F5) ─────────────────────────

// handleModelPricingGet returns the current per-model token pricing configuration.
func (s *Server) handleModelPricingGet(w http.ResponseWriter, r *http.Request) {
	prices, err := s.store.GetModelPricing()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"pricing": prices})
}

// handleModelPricingPut updates per-model token pricing configuration.
func (s *Server) handleModelPricingPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pricing []store.ModelPrice `json:"pricing"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if len(req.Pricing) == 0 {
		jsonError(w, "pricing array is required", http.StatusBadRequest)
		return
	}

	// Validate: every entry needs a modelId and non-negative prices
	for _, p := range req.Pricing {
		if p.ModelID == "" {
			jsonError(w, "each pricing entry requires a modelId", http.StatusBadRequest)
			return
		}
		if p.InputPer1M < 0 || p.OutputPer1M < 0 || p.CachePer1M < 0 {
			jsonError(w, "prices cannot be negative", http.StatusBadRequest)
			return
		}
	}

	if err := s.store.SetModelPricing(req.Pricing); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.store.LogInfo("ui", "pricing_updated", "", map[string]interface{}{
		"modelCount": len(req.Pricing),
	})

	writeJSON(w, map[string]interface{}{
		"message": "pricing updated",
		"pricing": req.Pricing,
	})
}

// ── Phase 14: Cost Tracking (F8) ─────────────────────────────────

// computeAccountCosts estimates dollar costs for each account using
// burn rates from the forecast engine + model pricing from F5.
func (s *Server) computeAccountCosts(
	snapshots []*client.Snapshot,
	forecasts map[int64][]forecast.GroupForecast,
) map[int64]costtrack.AccountCostEstimate {
	if len(snapshots) == 0 {
		return nil
	}

	// Load pricing and ceilings from config
	pricing, err := s.store.GetModelPricing()
	if err != nil {
		s.logger.Warn("cost tracking: pricing load failed", "error", err)
		return nil
	}

	ceilings, err := costtrack.ParseCeilings(s.store.GetQuotaCeilings())
	if err != nil {
		s.logger.Warn("cost tracking: ceilings parse failed", "error", err)
		ceilings = costtrack.DefaultQuotaCeilings()
	}

	// Convert store.ModelPrice to costtrack.ModelPricing
	ctPricing := make([]costtrack.ModelPricing, len(pricing))
	for i, p := range pricing {
		ctPricing[i] = costtrack.ModelPricing{
			ModelID:     p.ModelID,
			DisplayName: p.DisplayName,
			Provider:    p.Provider,
			InputPer1M:  p.InputPer1M,
			OutputPer1M: p.OutputPer1M,
			CachePer1M:  p.CachePer1M,
		}
	}

	assigner := func(modelID string) string {
		return client.GroupForModel(modelID, "")
	}

	result := make(map[int64]costtrack.AccountCostEstimate)

	for _, snap := range snapshots {
		if snap == nil || snap.AccountID == 0 {
			continue
		}

		// Build GroupRate from forecast data (if available) or from latest snapshot
		var rates []costtrack.GroupRate

		if forecastGroups, ok := forecasts[snap.AccountID]; ok && len(forecastGroups) > 0 {
			// Use forecast data (has burn rates)
			for _, fg := range forecastGroups {
				rates = append(rates, costtrack.GroupRate{
					GroupKey:   fg.GroupKey,
					BurnRate:  fg.BurnRate,
					Remaining: fg.Remaining,
					HasData:   fg.Confidence != "none",
				})
			}
		} else {
			// No forecast data — compute remaining from latest snapshot
			groupRemaining := map[string]struct {
				sum   float64
				count int
			}{}
			for _, m := range snap.Models {
				gk := client.GroupForModel(m.ModelID, m.Label)
				acc := groupRemaining[gk]
				acc.sum += m.RemainingFraction
				acc.count++
				groupRemaining[gk] = acc
			}
			for _, key := range client.GroupOrder {
				if acc, ok := groupRemaining[key]; ok && acc.count > 0 {
					rates = append(rates, costtrack.GroupRate{
						GroupKey:   key,
						Remaining: acc.sum / float64(acc.count),
						HasData:   true,
					})
				}
			}
		}

		if len(rates) == 0 {
			continue
		}

		est := costtrack.EstimateAccountCost(
			snap.AccountID, snap.Email,
			rates, ceilings, ctPricing, assigner,
		)
		result[snap.AccountID] = est
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// handleCost returns estimated costs for all tracked accounts.
func (s *Server) handleCost(w http.ResponseWriter, r *http.Request) {
	snapshots, err := s.store.LatestPerAccount()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Compute forecasts first (needed for burn rates)
	forecasts := s.computeAccountForecasts(snapshots)

	// Compute costs
	costs := s.computeAccountCosts(snapshots, forecasts)

	// Aggregate total
	var totalCost float64
	var accounts []costtrack.AccountCostEstimate
	for _, snap := range snapshots {
		if snap == nil {
			continue
		}
		if est, ok := costs[snap.AccountID]; ok {
			accounts = append(accounts, est)
			totalCost += est.TotalCost
		}
	}

	result := map[string]interface{}{
		"accounts":   accounts,
		"totalCost":  totalCost,
		"totalLabel": costtrack.FormatCost(totalCost),
	}

	// Include ceilings for transparency
	ceilings, _ := costtrack.ParseCeilings(s.store.GetQuotaCeilings())
	result["quotaCeilings"] = ceilings

	writeJSON(w, result)
}

// ── Phase 14: Forecast Handlers ──────────────────────────────────

// computeAccountForecasts builds sliding-window TTX forecasts for all accounts.
// Returns a map of accountID → []GroupForecast for inline enrichment in /api/status.
func (s *Server) computeAccountForecasts(snapshots []*client.Snapshot) map[int64][]forecast.GroupForecast {
	if len(snapshots) == 0 {
		return nil
	}

	groups := make([]forecast.GroupDefinition, len(client.GroupOrder))
	for i, key := range client.GroupOrder {
		groups[i] = forecast.GroupDefinition{
			GroupKey:     key,
			DisplayName: client.GroupDisplayNames[key],
		}
	}

	assigner := func(modelID string) string {
		return client.GroupForModel(modelID, "")
	}

	result := make(map[int64][]forecast.GroupForecast)

	for _, snap := range snapshots {
		if snap == nil || snap.AccountID == 0 {
			continue
		}

		// Get recent snapshots for this account (last 60 min)
		recent, err := s.store.RecentModelSnapshots(snap.AccountID, forecast.DefaultWindow)
		if err != nil || len(recent) < 2 {
			continue
		}

		// Convert to forecast.SnapshotPoint
		points := make([]forecast.SnapshotPoint, 0, len(recent))
		for _, r := range recent {
			models := forecast.ParseModelsJSON(r.ModelsJSON)
			if models != nil {
				points = append(points, forecast.SnapshotPoint{
					CapturedAt: r.CapturedAt,
					Models:     models,
				})
			}
		}

		if len(points) < 2 {
			continue
		}

		// Compute rates from recent history
		rates := forecast.ComputeRates(points)

		// Build current remaining + reset times from latest snapshot
		// Apply stale-correction: if a model's reset time is in the past,
		// the quota has refilled — infer remaining = 1.0 to match readiness display.
		remaining := make(map[string]float64)
		resetTimes := make(map[string]*time.Time)
		now := time.Now()
		for _, m := range snap.Models {
			frac := m.RemainingFraction
			if m.ResetTime != nil && m.ResetTime.Before(now) && frac <= 0 {
				// Reset time passed and model was exhausted → quota has refilled
				frac = 1.0
			}
			remaining[m.ModelID] = frac
			resetTimes[m.ModelID] = m.ResetTime
		}

		// Compute group-level forecasts
		gf := forecast.ComputeGroupForecasts(rates, remaining, resetTimes, assigner, groups)
		if len(gf) > 0 {
			result[snap.AccountID] = gf
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// handleForecast returns detailed TTX forecasts for all tracked providers.
func (s *Server) handleForecast(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{}

	// Antigravity account forecasts
	snapshots, err := s.store.LatestPerAccount()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	forecasts := s.computeAccountForecasts(snapshots)

	// Build enriched forecast with account context
	type accountForecast struct {
		AccountID int64                    `json:"accountId"`
		Email     string                   `json:"email"`
		PlanName  string                   `json:"planName"`
		Groups    []forecast.GroupForecast `json:"groups"`
	}

	var antigravityForecasts []accountForecast
	for _, snap := range snapshots {
		if snap == nil {
			continue
		}
		gf := forecasts[snap.AccountID]
		if gf == nil {
			continue
		}
		antigravityForecasts = append(antigravityForecasts, accountForecast{
			AccountID: snap.AccountID,
			Email:     snap.Email,
			PlanName:  snap.PlanName,
			Groups:    gf,
		})
	}
	if len(antigravityForecasts) > 0 {
		result["antigravity"] = antigravityForecasts
	}

	// Claude Code forecast
	claudeForecasts := s.computeClaudeForecasts()
	if claudeForecasts != nil {
		result["claude"] = claudeForecasts
	}

	// Codex forecast
	codexForecasts := s.computeCodexForecasts()
	if codexForecasts != nil {
		result["codex"] = codexForecasts
	}

	// Advisor: best account recommendation with TTX context
	if len(antigravityForecasts) > 1 {
		summariesByAccount := make(map[int64][]*tracker.UsageSummary)
		if s.tracker != nil {
			for _, snap := range snapshots {
				summaries, _ := s.tracker.AllUsageSummaries(snap, snap.AccountID)
				if len(summaries) > 0 {
					summariesByAccount[snap.AccountID] = summaries
				}
			}
		}
		rec := advisor.Recommend(snapshots, summariesByAccount)
		if rec != nil {
			result["advisor"] = rec
		}
	}

	writeJSON(w, result)
}

// computeClaudeForecasts computes TTX for Claude Code from recent snapshots.
func (s *Server) computeClaudeForecasts() map[string]interface{} {
	recent, err := s.store.RecentClaudeSnapshots(forecast.DefaultWindow)
	if err != nil || len(recent) < 2 {
		return nil
	}

	// Build snapshot points for 5-hour and 7-day windows
	type claudeForecast struct {
		Window   string  `json:"window"`
		BurnRate float64 `json:"burnRate"` // pct/hr
		TTXHours float64 `json:"ttxHours"`
		TTXLabel string  `json:"ttxLabel"`
		Severity string  `json:"severity"`
		Used     float64 `json:"used"`
	}

	var forecasts []claudeForecast

	// 5-hour window
	if rate, remaining := computeSimpleRate(recent, func(s store.ClaudeSnapshot) float64 {
		return 100 - s.FiveHourPct // FiveHourPct is % used, we need % remaining
	}); rate > 0 && remaining >= 0 {
		ttx := remaining / rate
		forecasts = append(forecasts, claudeForecast{
			Window:   "5-hour",
			BurnRate: rate,
			TTXHours: ttx,
			TTXLabel: forecast.FormatTTXPublic(ttx),
			Severity: ttxSeverity(ttx),
			Used:     100 - remaining,
		})
	}

	// 7-day window
	if rate, remaining := computeSimpleRate(recent, func(s store.ClaudeSnapshot) float64 {
		if s.SevenDayPct != nil {
			return 100 - *s.SevenDayPct
		}
		return -1 // no data
	}); rate > 0 && remaining >= 0 {
		ttx := remaining / rate
		forecasts = append(forecasts, claudeForecast{
			Window:   "7-day",
			BurnRate: rate,
			TTXHours: ttx,
			TTXLabel: forecast.FormatTTXPublic(ttx),
			Severity: ttxSeverity(ttx),
			Used:     100 - remaining,
		})
	}

	if len(forecasts) == 0 {
		return nil
	}
	return map[string]interface{}{
		"windows": forecasts,
	}
}

// computeCodexForecasts computes TTX for Codex from recent snapshots.
func (s *Server) computeCodexForecasts() map[string]interface{} {
	recent, err := s.store.RecentCodexSnapshots(forecast.DefaultWindow)
	if err != nil || len(recent) < 2 {
		return nil
	}

	type codexForecast struct {
		Window   string  `json:"window"`
		BurnRate float64 `json:"burnRate"`
		TTXHours float64 `json:"ttxHours"`
		TTXLabel string  `json:"ttxLabel"`
		Severity string  `json:"severity"`
		Used     float64 `json:"used"`
	}

	var forecasts []codexForecast

	// 5-hour window
	if rate, remaining := computeCodexRate(recent, func(s *store.CodexSnapshot) float64 {
		return 100 - s.FiveHourPct
	}); rate > 0 && remaining >= 0 {
		ttx := remaining / rate
		forecasts = append(forecasts, codexForecast{
			Window:   "5-hour",
			BurnRate: rate,
			TTXHours: ttx,
			TTXLabel: forecast.FormatTTXPublic(ttx),
			Severity: ttxSeverity(ttx),
			Used:     100 - remaining,
		})
	}

	if len(forecasts) == 0 {
		return nil
	}
	return map[string]interface{}{
		"windows": forecasts,
	}
}

// computeSimpleRate computes a weighted-average rate from Claude snapshots.
// Returns (rate in pct/hr, current remaining pct). Rate is 0 if no decrease detected.
func computeSimpleRate(snaps []store.ClaudeSnapshot, extractor func(store.ClaudeSnapshot) float64) (float64, float64) {
	if len(snaps) < 2 {
		return 0, -1
	}

	totalWeight := 0.0
	weightedRate := 0.0
	pairCount := 0

	for i := 1; i < len(snaps); i++ {
		prev := extractor(snaps[i-1])
		curr := extractor(snaps[i])
		if prev < 0 || curr < 0 {
			continue
		}

		dt := snaps[i].CapturedAt.Sub(snaps[i-1].CapturedAt)
		if dt <= 0 {
			continue
		}

		pairCount++
		consumed := prev - curr // positive = usage
		if consumed < 0 {
			consumed = 0 // reset or correction
		}

		rate := consumed / dt.Hours()
		weight := 1.0 + float64(i-1)/float64(len(snaps)-1)
		weightedRate += rate * weight
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return 0, -1
	}

	lastRemaining := extractor(snaps[len(snaps)-1])
	return weightedRate / totalWeight, lastRemaining
}

// computeCodexRate computes a weighted-average rate from Codex snapshots.
func computeCodexRate(snaps []*store.CodexSnapshot, extractor func(*store.CodexSnapshot) float64) (float64, float64) {
	if len(snaps) < 2 {
		return 0, -1
	}

	totalWeight := 0.0
	weightedRate := 0.0
	pairCount := 0

	for i := 1; i < len(snaps); i++ {
		prev := extractor(snaps[i-1])
		curr := extractor(snaps[i])
		if prev < 0 || curr < 0 {
			continue
		}

		dt := snaps[i].CapturedAt.Sub(snaps[i-1].CapturedAt)
		if dt <= 0 {
			continue
		}

		pairCount++
		consumed := prev - curr
		if consumed < 0 {
			consumed = 0
		}

		rate := consumed / dt.Hours()
		weight := 1.0 + float64(i-1)/float64(len(snaps)-1)
		weightedRate += rate * weight
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return 0, -1
	}

	lastRemaining := extractor(snaps[len(snaps)-1])
	return weightedRate / totalWeight, lastRemaining
}

// ttxSeverity returns the severity level for a given TTX in hours.
func ttxSeverity(hours float64) string {
	switch {
	case hours <= 0:
		return "critical"
	case hours < 0.5:
		return "critical"
	case hours < 1.0:
		return "warning"
	case hours < 3.0:
		return "caution"
	default:
		return "safe"
	}
}
