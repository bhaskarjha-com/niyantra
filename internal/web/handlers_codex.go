package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/codex"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

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
