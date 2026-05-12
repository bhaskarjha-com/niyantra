package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/advisor"
	"github.com/bhaskarjha-com/niyantra/internal/claude"
	"github.com/bhaskarjha-com/niyantra/internal/notify"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tracker"
)

// ── Operational Endpoints ─────────────────────────────────────────

// handleHealthz returns basic health/liveness data for monitoring.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"status":        "ok",
		"version":       s.Version,
		"uptime":        time.Since(s.startTime).Truncate(time.Second).String(),
		"schemaVersion": s.store.SchemaVersion(),
		"accounts":      s.store.AccountCount(),
		"snapshots":     s.store.SnapshotCount(),
	})
}

// ── Phase 9 Handlers ─────────────────────────────────────────────

// handleClaudeStatus returns the current Claude Code rate limit data.
func (s *Server) handleClaudeStatus(w http.ResponseWriter, r *http.Request) {
	bridgeEnabled := s.store.GetConfigBool("claude_bridge")
	installed := claude.IsClaudeCodeInstalled()
	fresh := claude.IsFresh(claude.DefaultStaleness)

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

// ── Phase 14: Claude Code Deep Tracking (F15d) ──────────────────

// handleClaudeUsage returns deep token usage analytics from Claude Code's
// local JSONL session files. Zero network calls — pure filesystem parsing.
func (s *Server) handleClaudeUsage(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 365 {
			days = v
		}
	}

	// Wire model pricing callback to F5 pricing config
	priceFn := func(modelID string) (float64, float64, float64, bool) {
		p := s.store.GetModelPrice(modelID)
		if p == nil {
			return 0, 0, 0, false
		}
		return p.InputPer1M, p.OutputPer1M, p.CachePer1M, true
	}

	summary, err := claude.AggregateUsage(days, priceFn)
	if err != nil {
		jsonError(w, fmt.Sprintf("failed to aggregate usage: %v", err), http.StatusInternalServerError)
		return
	}

	if summary == nil {
		summary = &claude.UsageSummary{
			Days: []claude.DailyUsage{},
		}
	}

	writeJSON(w, summary)
}

