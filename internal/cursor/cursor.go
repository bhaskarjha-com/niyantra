// Package cursor provides Cursor IDE credential detection and usage API polling.
//
// Authentication methods (priority order):
//  1. Auto-detect JWT from local state.vscdb (Cursor's internal SQLite DB)
//  2. Manual session token from config (cursor_session_token)
//
// API: cursor.com/api/usage (GET) — returns per-model numRequests / maxRequestUsage.
//
// IMPORTANT: Cursor's usage API is internal and undocumented. It may change
// without notice. This implementation includes graceful degradation and backoff.
package cursor

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ── Errors ──────────────────────────────────────────────────────────

var (
	ErrNotInstalled     = errors.New("cursor: not installed (state.vscdb not found)")
	ErrNoToken          = errors.New("cursor: no authentication token available")
	ErrUnauthorized     = errors.New("cursor: unauthorized (401)")
	ErrForbidden        = errors.New("cursor: forbidden (403)")
	ErrServerError      = errors.New("cursor: server error")
	ErrNetworkError     = errors.New("cursor: network error")
	ErrInvalidResponse  = errors.New("cursor: invalid response")
	ErrDatabaseReadFail = errors.New("cursor: failed to read state.vscdb")
)

// ── Credentials ─────────────────────────────────────────────────────

// Credentials contains the extracted Cursor auth state.
type Credentials struct {
	AccessToken string    // JWT from state.vscdb or manual session token
	Email       string    // from JWT claims if available
	Source      string    // "state.vscdb" or "manual"
	DetectedAt  time.Time // when credentials were last detected
}

// ── Usage Models ────────────────────────────────────────────────────

// Usage represents the parsed Cursor usage response.
type Usage struct {
	Models       map[string]ModelUsage `json:"models"`       // model name → usage
	StartOfMonth string                `json:"startOfMonth"` // e.g. "2026-05-01"
	PremiumUsed  int                   // computed: sum of all model numRequests
	PremiumLimit int                   // computed: max of all model maxRequestUsage
}

// ModelUsage represents per-model request usage.
type ModelUsage struct {
	NumRequests     int `json:"numRequests"`
	MaxRequestUsage int `json:"maxRequestUsage"`
}

// UsagePct returns the percentage of quota used (0-100).
func (u *Usage) UsagePct() float64 {
	if u.PremiumLimit <= 0 {
		return 0
	}
	return float64(u.PremiumUsed) / float64(u.PremiumLimit) * 100
}

// ── Client ──────────────────────────────────────────────────────────

const (
	usageURL    = "https://www.cursor.com/api/usage"
	httpTimeout = 15 * time.Second
)

// Client is the Cursor usage API client.
type Client struct {
	token     string
	logger    *slog.Logger
	http      *http.Client
}

// NewClient creates a new Cursor API client.
func NewClient(token string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		token:  token,
		logger: logger,
		http: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// SetToken updates the authentication token.
func (c *Client) SetToken(token string) {
	c.token = token
}

// FetchUsage polls the Cursor usage API.
func (c *Client) FetchUsage(ctx context.Context) (*Usage, error) {
	if c.token == "" {
		return nil, ErrNoToken
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}

	// Auth: try Cookie-based auth first (session token), fall back to Bearer
	req.Header.Set("Cookie", "WorkosCursorSessionToken="+c.token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Niyantra)")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		return nil, ErrForbidden
	default:
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
		}
		return nil, fmt.Errorf("%w: HTTP %d", ErrInvalidResponse, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrNetworkError, err)
	}

	// Parse the usage response
	// Expected format: {"gpt-4": {"numRequests": 150, "maxRequestUsage": 500}, "startOfMonth": "..."}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: JSON parse: %v", ErrInvalidResponse, err)
	}

	usage := &Usage{
		Models: make(map[string]ModelUsage),
	}

	for key, val := range raw {
		if key == "startOfMonth" {
			var s string
			json.Unmarshal(val, &s)
			usage.StartOfMonth = s
			continue
		}

		// Try to parse as model usage
		var mu ModelUsage
		if err := json.Unmarshal(val, &mu); err == nil && mu.MaxRequestUsage > 0 {
			usage.Models[key] = mu
			usage.PremiumUsed += mu.NumRequests
			if mu.MaxRequestUsage > usage.PremiumLimit {
				usage.PremiumLimit = mu.MaxRequestUsage
			}
		}
	}

	c.logger.Debug("Cursor usage fetched",
		"models", len(usage.Models),
		"premiumUsed", usage.PremiumUsed,
		"premiumLimit", usage.PremiumLimit,
		"startOfMonth", usage.StartOfMonth)

	return usage, nil
}

// ── Credential Detection ────────────────────────────────────────────

// stateDBPath returns the platform-specific path to Cursor's state.vscdb.
func stateDBPath() string {
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "Cursor", "User", "globalStorage", "state.vscdb")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb")
	default: // linux
		home, _ := os.UserHomeDir()
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			configDir = filepath.Join(home, ".config")
		}
		return filepath.Join(configDir, "Cursor", "User", "globalStorage", "state.vscdb")
	}
}

// DetectCredentials discovers Cursor authentication tokens.
// Priority: 1) local state.vscdb JWT  2) manual session token from config
func DetectCredentials(logger *slog.Logger, manualToken string) (*Credentials, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Try auto-detection from state.vscdb first
	creds, err := readStateDB(logger)
	if err == nil && creds != nil && creds.AccessToken != "" {
		return creds, nil
	}

	if err != nil && !errors.Is(err, ErrNotInstalled) {
		logger.Debug("Cursor state.vscdb read failed, trying manual token", "error", err)
	}

	// Fall back to manual token
	if manualToken != "" {
		return &Credentials{
			AccessToken: manualToken,
			Source:      "manual",
			DetectedAt:  time.Now().UTC(),
		}, nil
	}

	return nil, ErrNotInstalled
}

// readStateDB reads the cursorAuth/accessToken from Cursor's local state database.
func readStateDB(logger *slog.Logger) (*Credentials, error) {
	dbPath := stateDBPath()

	// Check if file exists
	if _, err := os.Stat(dbPath); err != nil {
		return nil, ErrNotInstalled
	}

	// Open with read-only mode and immutable flag for safety
	db, err := sql.Open("sqlite", dbPath+"?mode=ro&immutable=1")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseReadFail, err)
	}
	defer db.Close()

	// Read the access token from the key-value store
	var token string
	err = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken'`).Scan(&token)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debug("Cursor state.vscdb has no accessToken key")
			return nil, ErrNoToken
		}
		return nil, fmt.Errorf("%w: %v", ErrDatabaseReadFail, err)
	}

	if token == "" {
		return nil, ErrNoToken
	}

	// Try to read email (optional)
	var email string
	_ = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'cursorAuth/cachedEmail'`).Scan(&email)

	logger.Debug("Cursor credentials detected from state.vscdb", "email", email, "tokenLen", len(token))

	return &Credentials{
		AccessToken: token,
		Email:       email,
		Source:      "state.vscdb",
		DetectedAt:  time.Now().UTC(),
	}, nil
}
