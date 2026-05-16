package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/bhaskarjha-com/niyantra/internal/claude"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// handleConfigGet returns server configuration entries.
// Sensitive values (e.g., copilot_pat) are masked before transmission.
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	entries, err := s.store.AllConfig(category)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	// Mask sensitive config values
	for _, e := range entries {
		if e.Key == "copilot_pat" && e.Value != "" {
			e.Value = "configured"
		}
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

// onConfigChanged is called after a config key is updated via the API.
// Handles side effects for bridge, notification, and data source settings.
func (s *Server) onConfigChanged(key, value string) {
	switch key {
	case "claude_bridge":
		if value == "true" {
			if err := claude.SetupBridge(s.logger); err != nil {
				s.logger.Warn("Claude Code bridge setup failed", "error", err)
			}
		} else {
			if err := claude.DisableBridge(s.logger); err != nil {
				s.logger.Warn("Claude Code bridge disable failed", "error", err)
			}
		}
		// S7: Sync data_sources.claude_code.enabled to match config
		s.store.SetSourceEnabled("claude_code", value == "true")
	case "codex_capture":
		// S7: Sync data_sources.codex.enabled to match config
		s.store.SetSourceEnabled("codex", value == "true")
	case "copilot_capture":
		// F15c: Sync data_sources.copilot.enabled to match config
		s.store.SetSourceEnabled("copilot", value == "true")
	case "notify_enabled", "notify_threshold":
		s.notifier.Configure(
			s.store.GetConfigBool("notify_enabled"),
			s.store.GetConfigFloat("notify_threshold", 10),
		)
	}
}
