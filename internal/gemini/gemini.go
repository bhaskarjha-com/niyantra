// Package gemini provides Gemini CLI credential detection and usage API polling.
//
// Based on deep analysis of cclimits (cruzanstx/cclimits) source code:
//
// Two endpoints are polled sequentially:
//  1. LoadCodeAssist:     POST cloudcode-pa.googleapis.com/v1internal:loadCodeAssist
//     → returns currentTier and cloudaicompanionProject
//  2. RetrieveUserQuota:  POST cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota
//     → returns per-model buckets {modelId, remainingFraction, resetTime}
//
// Credentials are read from ~/.gemini/oauth_creds.json (access_token, refresh_token, expiry_date).
// Token auto-refresh uses https://oauth2.googleapis.com/token with CLIENT_ID/CLIENT_SECRET
// extracted from the Gemini CLI npm package or configured manually.
package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ── Errors ──────────────────────────────────────────────────────────

var (
	ErrNotInstalled    = errors.New("gemini: not installed")
	ErrNoToken         = errors.New("gemini: no access token")
	ErrTokenExpired    = errors.New("gemini: token expired and refresh failed")
	ErrUnauthorized    = errors.New("gemini: unauthorized (401)")
	ErrForbidden       = errors.New("gemini: forbidden (403)")
	ErrServerError     = errors.New("gemini: server error")
	ErrNetworkError    = errors.New("gemini: network error")
	ErrInvalidResponse = errors.New("gemini: invalid response")
	ErrNoProject       = errors.New("gemini: no companion project found")
)

// ── Credentials ─────────────────────────────────────────────────────

// Credentials holds Gemini CLI auth state extracted from local files.
type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiryDate   int64  `json:"expiry_date"` // Unix ms
	Email        string `json:"-"`            // from userinfo (populated lazily)
	Source       string `json:"-"`            // "auto" or "manual"
	FilePath     string `json:"-"`            // path to oauth_creds.json
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	if c.ExpiryDate <= 0 {
		return false // no expiry info — assume valid
	}
	// Convert ms to time. Add 60s buffer so we refresh before actual expiry.
	expiry := time.UnixMilli(c.ExpiryDate)
	return time.Now().After(expiry.Add(-60 * time.Second))
}

// ── API Response Types ──────────────────────────────────────────────

// loadCodeAssistResponse is the response from POST loadCodeAssist.
type loadCodeAssistResponse struct {
	CurrentTier             map[string]interface{} `json:"currentTier"`
	CloudAICompanionProject string                 `json:"cloudaicompanionProject"`
}

// quotaBucket is a single model quota bucket from retrieveUserQuota.
type quotaBucket struct {
	ModelID           string  `json:"modelId"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

// retrieveUserQuotaResponse is the response from POST retrieveUserQuota.
type retrieveUserQuotaResponse struct {
	Buckets []quotaBucket `json:"buckets"`
}

// ModelQuota represents a single model's quota state.
type ModelQuota struct {
	ModelID           string  `json:"modelId"`
	RemainingFraction float64 `json:"remainingFraction"` // 0.0 – 1.0
	UsedPct           float64 `json:"usedPct"`           // 0 – 100
	RemainingPct      float64 `json:"remainingPct"`      // 0 – 100
	ResetTime         string  `json:"resetTime,omitempty"`
	Tier              string  `json:"tier"` // "flash", "pro", "3-flash"
}

// Snapshot is the merged result from the two API calls.
type Snapshot struct {
	Tier      string       `json:"tier"`      // e.g. "standard", "enterprise"
	ProjectID string       `json:"projectId"` // cloudaicompanionProject
	Models    []ModelQuota `json:"models"`
	Email     string       `json:"email,omitempty"`
	// Aggregate usage (weighted avg of all models)
	OverallUsedPct float64 `json:"overallUsedPct"`
}

// ── Endpoints ───────────────────────────────────────────────────────

const (
	loadCodeAssistURL     = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	retrieveUserQuotaURL  = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
	tokenRefreshURL       = "https://oauth2.googleapis.com/token"
	userInfoURL           = "https://www.googleapis.com/oauth2/v1/userinfo"
	httpTimeout           = 15 * time.Second
)

// ── Model Tier Mapping ──────────────────────────────────────────────

// modelTier returns the tier name for a given model ID.
func modelTier(modelID string) string {
	lower := strings.ToLower(modelID)
	switch {
	case strings.Contains(lower, "3-flash"):
		return "3-flash"
	case strings.Contains(lower, "3-pro"):
		return "pro"
	case strings.Contains(lower, "flash-lite"):
		return "flash"
	case strings.Contains(lower, "flash"):
		return "flash"
	case strings.Contains(lower, "pro"):
		return "pro"
	default:
		return "other"
	}
}

// ── Client ──────────────────────────────────────────────────────────

// Client polls Gemini's Cloud Code Assist quota endpoints.
type Client struct {
	creds        *Credentials
	logger       *slog.Logger
	http         *http.Client
	clientID     string // OAuth client ID for token refresh
	clientSecret string // OAuth client secret for token refresh
}

// NewClient creates a Gemini API client.
func NewClient(creds *Credentials, logger *slog.Logger, clientID, clientSecret string) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		creds:        creds,
		logger:       logger,
		http:         &http.Client{Timeout: httpTimeout},
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// FetchSnapshot polls the two endpoints and builds a Snapshot.
func (c *Client) FetchSnapshot(ctx context.Context) (*Snapshot, error) {
	if c.creds.AccessToken == "" {
		return nil, ErrNoToken
	}

	// Auto-refresh if token is expired
	if c.creds.IsExpired() && c.creds.RefreshToken != "" {
		if err := c.refreshToken(ctx); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTokenExpired, err)
		}
	}

	// Step 1: loadCodeAssist → tier + project ID
	loadResp, err := c.fetchLoadCodeAssist(ctx)
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{
		ProjectID: loadResp.CloudAICompanionProject,
	}

	// Extract tier name
	if loadResp.CurrentTier != nil {
		if name, ok := loadResp.CurrentTier["name"].(string); ok {
			snap.Tier = name
		} else if id, ok := loadResp.CurrentTier["id"].(string); ok {
			snap.Tier = id
		}
	}
	if snap.Tier == "" {
		snap.Tier = "unknown"
	}

	// Step 2: retrieveUserQuota → per-model buckets
	if snap.ProjectID != "" {
		quotaResp, err := c.fetchUserQuota(ctx, snap.ProjectID)
		if err != nil {
			c.logger.Debug("Gemini quota endpoint failed (continuing with partial data)", "error", err)
		} else if len(quotaResp.Buckets) > 0 {
			var totalUsed float64
			for _, b := range quotaResp.Buckets {
				usedPct := (1 - b.RemainingFraction) * 100
				remainPct := b.RemainingFraction * 100
				mq := ModelQuota{
					ModelID:           b.ModelID,
					RemainingFraction: b.RemainingFraction,
					UsedPct:           usedPct,
					RemainingPct:      remainPct,
					ResetTime:         b.ResetTime,
					Tier:              modelTier(b.ModelID),
				}
				snap.Models = append(snap.Models, mq)
				totalUsed += usedPct
			}
			if len(snap.Models) > 0 {
				snap.OverallUsedPct = totalUsed / float64(len(snap.Models))
			}
		}
	}

	// Try to get email from userinfo (best-effort)
	if email := c.fetchEmail(ctx); email != "" {
		snap.Email = email
		c.creds.Email = email
	}

	c.logger.Info("Gemini snapshot fetched",
		"tier", snap.Tier, "models", len(snap.Models),
		"overallUsedPct", snap.OverallUsedPct)

	return snap, nil
}

// fetchLoadCodeAssist calls POST loadCodeAssist.
func (c *Client) fetchLoadCodeAssist(ctx context.Context) (*loadCodeAssistResponse, error) {
	body := `{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loadCodeAssistURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Niyantra)")

	var result loadCodeAssistResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// fetchUserQuota calls POST retrieveUserQuota.
func (c *Client) fetchUserQuota(ctx context.Context, projectID string) (*retrieveUserQuotaResponse, error) {
	bodyJSON := fmt.Sprintf(`{"project":"%s"}`, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, retrieveUserQuotaURL, strings.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Niyantra)")

	var result retrieveUserQuotaResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// fetchEmail gets the user's email from the Google userinfo endpoint (best-effort).
func (c *Client) fetchEmail(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)

	var info struct {
		Email string `json:"email"`
	}
	if err := c.doJSON(req, &info); err != nil {
		return ""
	}
	return info.Email
}

// refreshToken refreshes the OAuth access token using the refresh_token.
func (c *Client) refreshToken(ctx context.Context) error {
	if c.clientID == "" || c.clientSecret == "" {
		return fmt.Errorf("no OAuth client credentials configured for token refresh")
	}

	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {c.creds.RefreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenRefreshURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh failed: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read refresh response: %v", err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return fmt.Errorf("parse refresh response: %v", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("empty access_token in refresh response")
	}

	// Update in-memory credentials
	c.creds.AccessToken = tokenResp.AccessToken
	newExpiryMs := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()
	c.creds.ExpiryDate = newExpiryMs

	// Persist refreshed token back to file
	if c.creds.FilePath != "" {
		if err := persistRefreshedToken(c.creds.FilePath, tokenResp.AccessToken, newExpiryMs); err != nil {
			c.logger.Warn("Failed to persist refreshed Gemini token", "error", err)
			// Non-fatal: in-memory token still works for this session
		} else {
			c.logger.Debug("Gemini token refreshed and persisted")
		}
	}

	return nil
}

// persistRefreshedToken updates the oauth_creds.json file with the new token.
func persistRefreshedToken(filePath, accessToken string, expiryMs int64) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	obj["access_token"] = accessToken
	obj["expiry_date"] = expiryMs

	updated, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file then rename
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, updated, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, filePath)
}

func (c *Client) doJSON(req *http.Request, out interface{}) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
		}
		return fmt.Errorf("%w: HTTP %d", ErrInvalidResponse, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read: %v", ErrNetworkError, err)
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("%w: JSON: %v", ErrInvalidResponse, err)
	}
	return nil
}

// ── Credential Detection ────────────────────────────────────────────

// geminiDataDir returns the platform-specific Gemini CLI config root.
func geminiDataDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	// All platforms use ~/.gemini
	return filepath.Join(home, ".gemini")
}

// DetectCredentials discovers OAuth credentials from Gemini CLI's local files.
func DetectCredentials(logger *slog.Logger) (*Credentials, error) {
	if logger == nil {
		logger = slog.Default()
	}

	dataDir := geminiDataDir()
	if dataDir == "" {
		return nil, ErrNotInstalled
	}

	oauthPath := filepath.Join(dataDir, "oauth_creds.json")
	if _, err := os.Stat(oauthPath); err != nil {
		return nil, fmt.Errorf("%w: %s not found", ErrNotInstalled, oauthPath)
	}

	data, err := os.ReadFile(oauthPath)
	if err != nil {
		return nil, fmt.Errorf("%w: read oauth_creds.json: %v", ErrNoToken, err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("%w: parse oauth_creds.json: %v", ErrNoToken, err)
	}

	if creds.AccessToken == "" {
		return nil, ErrNoToken
	}

	creds.Source = "auto"
	creds.FilePath = oauthPath

	logger.Debug("Gemini credentials detected",
		"source", creds.Source,
		"tokenLen", len(creds.AccessToken),
		"hasRefresh", creds.RefreshToken != "",
		"expired", creds.IsExpired())

	return &creds, nil
}

// ── OAuth Client Credentials ────────────────────────────────────────

// ExtractOAuthClientCreds attempts to find the Gemini CLI's OAuth CLIENT_ID and
// CLIENT_SECRET from the npm installation. These are public installed-app
// credentials, not secrets.
func ExtractOAuthClientCreds(logger *slog.Logger) (clientID, clientSecret string) {
	if logger == nil {
		logger = slog.Default()
	}

	// Check environment variables first
	clientID = os.Getenv("GEMINI_OAUTH_CLIENT_ID")
	clientSecret = os.Getenv("GEMINI_OAUTH_CLIENT_SECRET")
	if clientID != "" && clientSecret != "" {
		return clientID, clientSecret
	}

	// Try common installation paths
	home, _ := os.UserHomeDir()
	if home == "" {
		return "", ""
	}

	var candidates []string
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		candidates = append(candidates,
			filepath.Join(appdata, "npm", "node_modules", "@google", "gemini-cli-core", "dist", "src", "code_assist", "oauth2.js"),
			filepath.Join(appdata, "npm", "node_modules", "@google", "gemini-cli", "node_modules", "@google", "gemini-cli-core", "dist", "src", "code_assist", "oauth2.js"),
		)
	default:
		candidates = append(candidates,
			"/usr/local/lib/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
			"/usr/local/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
		)
	}

	// Also check npx cache directories
	npmDir := filepath.Join(home, ".npm", "_npx")
	if entries, err := os.ReadDir(npmDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				p := filepath.Join(npmDir, e.Name(), "node_modules", "@google", "gemini-cli-core", "dist", "src", "code_assist", "oauth2.js")
				candidates = append(candidates, p)
				p2 := filepath.Join(npmDir, e.Name(), "node_modules", "@google", "gemini-cli", "node_modules", "@google", "gemini-cli-core", "dist", "src", "code_assist", "oauth2.js")
				candidates = append(candidates, p2)
			}
		}
	}

	for _, path := range candidates {
		id, secret := extractFromOAuth2JS(path)
		if id != "" && secret != "" {
			logger.Debug("Gemini OAuth client creds extracted", "path", path)
			return id, secret
		}
	}

	return "", ""
}

// extractFromOAuth2JS parses CLIENT_ID and CLIENT_SECRET from oauth2.js.
func extractFromOAuth2JS(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	content := string(data)

	var clientID, clientSecret string

	// Look for CLIENT_ID = "..." or CLIENT_ID = '...'
	for _, prefix := range []string{`CLIENT_ID = "`, `CLIENT_ID = '`, `CLIENT_ID="`, `CLIENT_ID='`} {
		if idx := strings.Index(content, prefix); idx >= 0 {
			start := idx + len(prefix)
			end := strings.IndexAny(content[start:], `"'`)
			if end > 0 {
				clientID = content[start : start+end]
				break
			}
		}
	}

	for _, prefix := range []string{`CLIENT_SECRET = "`, `CLIENT_SECRET = '`, `CLIENT_SECRET="`, `CLIENT_SECRET='`} {
		if idx := strings.Index(content, prefix); idx >= 0 {
			start := idx + len(prefix)
			end := strings.IndexAny(content[start:], `"'`)
			if end > 0 {
				clientSecret = content[start : start+end]
				break
			}
		}
	}

	return clientID, clientSecret
}
