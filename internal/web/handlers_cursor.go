package web

import (
	"fmt"
	"net/http"

	"github.com/bhaskarjha-com/niyantra/internal/cursor"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// handleCursorStatus returns Cursor detection state and latest snapshot.
func (s *Server) handleCursorStatus(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"installed":      false,
		"captureEnabled": s.store.GetConfigBool("cursor_capture"),
	}

	manualToken := s.store.GetConfig("cursor_session_token")
	creds, err := cursor.DetectCredentials(s.logger, manualToken)
	if err == nil && creds != nil {
		result["installed"] = true
		result["email"] = creds.Email
		result["userId"] = creds.UserID
		result["source"] = creds.Source
	}

	snap, err := s.store.LatestCursorSnapshot()
	if err != nil {
		s.logger.Error("Failed to get Cursor snapshot", "error", err)
	}
	if snap != nil {
		result["snapshot"] = snap
	}

	writeJSON(w, result)
}

// handleCursorSnap triggers a manual Cursor usage snapshot.
func (s *Server) handleCursorSnap(w http.ResponseWriter, r *http.Request) {
	manualToken := s.store.GetConfig("cursor_session_token")
	creds, err := cursor.DetectCredentials(s.logger, manualToken)
	if err != nil {
		jsonError(w, fmt.Sprintf("Cursor not detected: %v", err), http.StatusBadRequest)
		return
	}

	client := cursor.NewClient(creds, s.logger)
	snapshot, err := client.FetchSnapshot(r.Context())
	if err != nil {
		jsonError(w, fmt.Sprintf("Cursor API error: %v", err), http.StatusBadGateway)
		return
	}

	snap := &store.CursorSnapshot{
		Email:         creds.Email,
		BillingModel:  snapshot.BillingModel,
		PlanTier:      snapshot.PlanTier,
		RequestsUsed:  snapshot.RequestsUsed,
		RequestsMax:   snapshot.RequestsMax,
		UsedCents:     snapshot.UsedCents,
		LimitCents:    snapshot.LimitCents,
		UsagePct:      snapshot.UsagePct(),
		AutoPct:       snapshot.AutoPercentUsed,
		APIPct:        snapshot.APIPercentUsed,
		CycleStart:    snapshot.CycleStart,
		CycleEnd:      snapshot.CycleEnd,
		CaptureMethod: "manual",
		CaptureSource: "ui",
	}

	if creds.Email != "" {
		accountID, err := s.store.GetOrCreateAccount(creds.Email, "Cursor", "cursor")
		if err == nil {
			snap.AccountID = accountID
		}
	}

	snapID, err := s.store.InsertCursorSnapshot(snap)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	s.store.UpdateSourceCapture("cursor")
	s.store.LogInfo("ui", "cursor_snap", creds.Email, map[string]interface{}{
		"billing": snapshot.BillingModel, "plan": snapshot.PlanTier,
		"usagePct": snapshot.UsagePct(), "method": "manual",
	})

	writeJSON(w, map[string]interface{}{
		"message":      "Cursor snapshot captured",
		"snapshotId":   snapID,
		"billingModel": snapshot.BillingModel,
		"planTier":     snapshot.PlanTier,
		"usagePct":     snapshot.UsagePct(),
		"requestsUsed": snapshot.RequestsUsed,
		"requestsMax":  snapshot.RequestsMax,
		"usedCents":    snapshot.UsedCents,
		"limitCents":   snapshot.LimitCents,
	})
}
