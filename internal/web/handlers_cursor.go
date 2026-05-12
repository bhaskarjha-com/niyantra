package web

import (
	"encoding/json"
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

	// Detect credentials
	manualToken := s.store.GetConfig("cursor_session_token")
	creds, err := cursor.DetectCredentials(s.logger, manualToken)
	if err == nil && creds != nil {
		result["installed"] = true
		result["email"] = creds.Email
		result["source"] = creds.Source
	}

	// Latest snapshot
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

	// Fetch usage
	client := cursor.NewClient(creds.AccessToken, s.logger)
	usage, err := client.FetchUsage(r.Context())
	if err != nil {
		jsonError(w, fmt.Sprintf("Cursor API error: %v", err), http.StatusBadGateway)
		return
	}

	// Build models JSON
	modelsJSON, _ := json.Marshal(usage.Models)

	// Build and store snapshot
	snap := &store.CursorSnapshot{
		Email:         creds.Email,
		PremiumUsed:   usage.PremiumUsed,
		PremiumLimit:  usage.PremiumLimit,
		UsagePct:      usage.UsagePct(),
		StartOfMonth:  usage.StartOfMonth,
		ModelsJSON:    string(modelsJSON),
		CaptureMethod: "manual",
		CaptureSource: "ui",
	}

	// Try to get or create an account for this cursor user
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

	// Update data source bookkeeping
	s.store.UpdateSourceCapture("cursor")

	// Log the snap
	s.store.LogInfo("ui", "cursor_snap", creds.Email, map[string]interface{}{
		"premiumUsed": usage.PremiumUsed, "premiumLimit": usage.PremiumLimit,
		"method": "manual", "models": len(usage.Models),
	})

	writeJSON(w, map[string]interface{}{
		"message":      "Cursor snapshot captured",
		"snapshotId":   snapID,
		"premiumUsed":  usage.PremiumUsed,
		"premiumLimit": usage.PremiumLimit,
		"usagePct":     usage.UsagePct(),
		"models":       usage.Models,
	})
}
