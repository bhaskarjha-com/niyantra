package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/agent"
	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

//go:embed static/*
var staticFiles embed.FS

// Server is the Niyantra HTTP server.
type Server struct {
	logger   *slog.Logger
	store    *store.Store
	client   *client.Client
	port     int
	auth     string // "user:pass" or ""
	agentMgr *agent.Manager
}

// NewServer creates a new Niyantra web server.
func NewServer(logger *slog.Logger, s *store.Store, c *client.Client, port int, auth string) *Server {
	srv := &Server{
		logger:   logger,
		store:    s,
		client:   c,
		port:     port,
		auth:     auth,
		agentMgr: agent.NewManager(logger),
	}

	// Auto-start polling agent if config says so
	if s.GetConfigBool("auto_capture") {
		srv.startPollingAgent()
	}

	return srv
}

// startPollingAgent creates and starts the auto-capture polling agent.
func (s *Server) startPollingAgent() {
	interval := s.store.GetConfigInt("poll_interval", 300)
	if interval < 30 {
		interval = 30
	}

	ag := agent.NewPollingAgent(s.client, s.store, time.Duration(interval)*time.Second, s.logger)
	ag.SetPollingCheck(func() bool {
		return s.store.GetConfigBool("auto_capture")
	})

	s.agentMgr.Start(ag)
	s.logger.Info("Auto-capture started", "interval", interval)
}

// stopPollingAgent stops the auto-capture polling agent.
func (s *Server) stopPollingAgent() {
	s.agentMgr.Stop()
	s.logger.Info("Auto-capture stopped")
}

// Shutdown stops the agent and cleans up resources.
func (s *Server) Shutdown() {
	s.agentMgr.Stop()
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// Quota API routes (auto-tracked)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/snap", s.handleSnap)
	mux.HandleFunc("/api/history", s.handleHistory)

	// Subscription API routes (manual tracking)
	mux.HandleFunc("/api/subscriptions", s.handleSubscriptions)
	mux.HandleFunc("/api/subscriptions/", s.handleSubscriptionByID)
	mux.HandleFunc("/api/overview", s.handleOverview)
	mux.HandleFunc("/api/presets", s.handlePresets)
	mux.HandleFunc("/api/export/csv", s.handleExportCSV)

	// Config & infrastructure routes (v3)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/activity", s.handleActivity)
	mux.HandleFunc("/api/mode", s.handleMode)

	// Static files (embedded)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("web: embedded fs: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	var handler http.Handler = mux
	if s.auth != "" {
		handler = s.basicAuth(mux)
	}

	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, handler)
}

// handleStatus returns readiness for all accounts.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshots, err := s.store.LatestPerAccount()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	accounts := readiness.Calculate(snapshots, 0.0)

	writeJSON(w, map[string]interface{}{
		"accounts":      accounts,
		"snapshotCount": s.store.SnapshotCount(),
		"accountCount":  s.store.AccountCount(),
	})
}

// handleSnap triggers a snapshot capture.
func (s *Server) handleSnap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
			URL:           "https://windsurf.com",
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

	// Return updated accounts
	snapshots, _ := s.store.LatestPerAccount()
	accounts := readiness.Calculate(snapshots, 0.0)

	writeJSON(w, map[string]interface{}{
		"message":      "snapshot captured",
		"email":        snap.Email,
		"planName":     snap.PlanName,
		"snapshotId":   snapID,
		"accountId":    accountID,
		"accounts":     accounts,
		"accountCount": s.store.AccountCount(),
		"snapshotCount": s.store.SnapshotCount(),
	})
}

// handleHistory returns snapshot history.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
		ID            int64                `json:"id"`
		AccountID     int64                `json:"accountId"`
		Email         string               `json:"email"`
		CapturedAt    time.Time            `json:"capturedAt"`
		PlanName      string               `json:"planName"`
		Groups        []client.GroupedQuota `json:"groups"`
		CaptureMethod string               `json:"captureMethod"`
		CaptureSource string               `json:"captureSource"`
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

// handleConfig handles GET (list) and PUT (update) for server configuration.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		category := r.URL.Query().Get("category")
		entries, err := s.store.AllConfig(category)
		if err != nil {
			jsonError(w, "database error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"config": entries})

	case http.MethodPut:
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

		// React to poll_interval change while running
		if req.Key == "poll_interval" && s.agentMgr.IsRunning() {
			s.stopPollingAgent()
			s.startPollingAgent()
		}

		entries, _ := s.store.AllConfig("")
		writeJSON(w, map[string]interface{}{"config": entries})

	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleActivity returns recent activity log entries.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
		"mode":         mode,
		"autoCapture":  autoCapture,
		"isPolling":    s.agentMgr.IsRunning(),
		"pollInterval": s.store.GetConfigInt("poll_interval", 300),
		"sources":      sources,
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

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
