package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tracker"
)

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

	// F15a: Include Cursor snapshot if available
	cursorSnap, _ := s.store.LatestCursorSnapshot()
	if cursorSnap != nil {
		result["cursorSnapshot"] = cursorSnap
	}

	// F15b: Include Gemini CLI snapshot if available
	geminiSnap, _ := s.store.LatestGeminiSnapshot()
	if geminiSnap != nil {
		result["geminiSnapshot"] = geminiSnap
	}

	// F15c: Include Copilot snapshot if available
	copilotSnap, _ := s.store.LatestCopilotSnapshot()
	if copilotSnap != nil {
		result["copilotSnapshot"] = copilotSnap
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

	accountID, err := s.store.GetOrCreateAccount(snap.Email, snap.PlanName, "antigravity")
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
