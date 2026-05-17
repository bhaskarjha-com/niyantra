package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/store"
)

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

// handleAccountGet returns a single account by ID.
// Currently not needed by the frontend, reserved for future use.
func (s *Server) handleAccountGet(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || accountID <= 0 {
		jsonError(w, "invalid account ID", http.StatusBadRequest)
		return
	}

	account, err := s.store.GetAccountByID(accountID)
	if err != nil {
		jsonError(w, "account not found", http.StatusNotFound)
		return
	}
	writeJSON(w, account)
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
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
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

	// Fetch the specific snapshot by ID (replaces O(N) History scan)
	targetSnap, err := s.store.GetSnapshotByID(req.SnapshotID)
	if err != nil {
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

	// Reset notification suppression — user manually intervened,
	// so any "already notified" state is stale. Re-arm all alerts.
	if s.notifier != nil {
		s.notifier.ResetAllGuards()
	}

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

// ── Phase 14: Activity Heatmap (F6) ─────────────────────────────

// handleHeatmap returns daily snapshot counts across all providers
// for rendering a GitHub-style contribution calendar.
func (s *Server) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	days := 365
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 730 {
			days = v
		}
	}

	data, err := s.store.HeatmapData(days)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if data == nil {
		data = []store.HeatmapDay{}
	}

	// Build lookup map for streak calculation
	dayMap := make(map[string]bool, len(data))
	totalSnapshots := 0
	maxCount := 0
	for _, d := range data {
		dayMap[d.Date] = true
		totalSnapshots += d.Count
		if d.Count > maxCount {
			maxCount = d.Count
		}
	}

	activeDays := len(data)

	// Compute current streak and longest streak by walking dates
	now := time.Now().UTC()
	streak := 0
	longestStreak := 0
	currentStreak := 0

	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		if dayMap[d] {
			currentStreak++
			if currentStreak > longestStreak {
				longestStreak = currentStreak
			}
		} else if i == 0 {
			// Today has no activity — skip and try from yesterday
			continue
		} else {
			// First gap ends the current streak
			if streak == 0 {
				streak = currentStreak
			}
			currentStreak = 0
		}
	}
	// If we never hit a gap, streak = currentStreak
	if streak == 0 {
		streak = currentStreak
	}

	writeJSON(w, map[string]interface{}{
		"days":           data,
		"maxCount":       maxCount,
		"totalSnapshots": totalSnapshots,
		"activeDays":     activeDays,
		"streak":         streak,
		"longestStreak":  longestStreak,
	})
}
