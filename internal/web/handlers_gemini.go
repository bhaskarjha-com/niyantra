package web

import (
	"fmt"
	"net/http"

	"github.com/bhaskarjha-com/niyantra/internal/gemini"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// handleGeminiStatus returns Gemini CLI detection state and latest snapshot.
func (s *Server) handleGeminiStatus(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"installed":      false,
		"captureEnabled": s.store.GetConfigBool("gemini_capture"),
	}

	creds, err := gemini.DetectCredentials(s.logger)
	if err == nil && creds != nil {
		result["installed"] = true
		result["email"] = creds.Email
		result["source"] = creds.Source
		result["expired"] = creds.IsExpired()
		result["hasRefreshToken"] = creds.RefreshToken != ""
	}

	snap, err := s.store.LatestGeminiSnapshot()
	if err != nil {
		s.logger.Error("Failed to get Gemini snapshot", "error", err)
	}
	if snap != nil {
		result["snapshot"] = snap
	}

	writeJSON(w, result)
}

// handleGeminiSnap triggers a manual Gemini CLI usage snapshot.
func (s *Server) handleGeminiSnap(w http.ResponseWriter, r *http.Request) {
	creds, err := gemini.DetectCredentials(s.logger)
	if err != nil {
		jsonError(w, fmt.Sprintf("Gemini CLI not detected: %v", err), http.StatusBadRequest)
		return
	}

	clientID := s.store.GetConfig("gemini_client_id")
	clientSecret := s.store.GetConfig("gemini_client_secret")
	if clientID == "" || clientSecret == "" {
		clientID, clientSecret = gemini.ExtractOAuthClientCreds(s.logger)
	}

	client := gemini.NewClient(creds, s.logger, clientID, clientSecret)
	snapshot, err := client.FetchSnapshot(r.Context())
	if err != nil {
		jsonError(w, fmt.Sprintf("Gemini API error: %v", err), http.StatusBadGateway)
		return
	}

	snap := &store.GeminiSnapshot{
		Email:         snapshot.Email,
		Tier:          snapshot.Tier,
		OverallPct:    snapshot.OverallUsedPct,
		ModelsJSON:    store.FormatGeminiModelsJSON(snapshot.Models),
		ProjectID:     snapshot.ProjectID,
		CaptureMethod: "manual",
		CaptureSource: "ui",
	}

	if snapshot.Email != "" {
		accountID, err := s.store.GetOrCreateAccount(snapshot.Email, "Gemini CLI", "gemini")
		if err == nil {
			snap.AccountID = accountID
		}
	}

	snapID, err := s.store.InsertGeminiSnapshot(snap)
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}

	s.store.UpdateSourceCapture("gemini")
	s.store.LogInfo("ui", "gemini_snap", snapshot.Email, map[string]interface{}{
		"tier": snapshot.Tier, "models": len(snapshot.Models),
		"overallUsedPct": snapshot.OverallUsedPct, "method": "manual",
	})

	writeJSON(w, map[string]interface{}{
		"message":        "Gemini CLI snapshot captured",
		"snapshotId":     snapID,
		"tier":           snapshot.Tier,
		"overallUsedPct": snapshot.OverallUsedPct,
		"modelCount":     len(snapshot.Models),
	})
}
