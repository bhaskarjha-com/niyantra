// Package cursor provides Cursor IDE credential detection and usage API polling.
//
// Based on deep analysis of community tools (Tendo33/cursor-usage-tracker, CodexBar):
//
// Three endpoints are polled in parallel:
//  1. Legacy:  GET  cursor.com/api/usage?user=<userId>         (Cookie auth)
//  2. Usage:   POST api2.cursor.sh/.../GetCurrentPeriodUsage   (Bearer auth)
//  3. Stripe:  GET  cursor.com/api/auth/stripe                 (Cookie auth)
//
// Auth cookie format: WorkosCursorSessionToken=<userId>%3A%3A<accessToken>
// Credentials are read from state.vscdb (cursorAuth/accessToken + cursorAuth/cachedSignUpType)
// and userId from sentry/scope_v3.json or storage.json files.
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
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ── Errors ──────────────────────────────────────────────────────────

var (
	ErrNotInstalled    = errors.New("cursor: not installed")
	ErrNoToken         = errors.New("cursor: no access token")
	ErrNoUserID        = errors.New("cursor: no user ID found")
	ErrUnauthorized    = errors.New("cursor: unauthorized (401)")
	ErrForbidden       = errors.New("cursor: forbidden (403)")
	ErrServerError     = errors.New("cursor: server error")
	ErrNetworkError    = errors.New("cursor: network error")
	ErrInvalidResponse = errors.New("cursor: invalid response")
)

// ── Credentials ─────────────────────────────────────────────────────

// Credentials holds Cursor auth state extracted from local files.
type Credentials struct {
	UserID      string // user_xxx from sentry/scope_v3.json
	AccessToken string // JWT from state.vscdb
	Email       string // from state.vscdb (optional)
	Source      string // "auto" or "manual"
}

// SessionCookie builds the auth cookie value: userId%3A%3AaccessToken
func (c *Credentials) SessionCookie() string {
	return fmt.Sprintf("WorkosCursorSessionToken=%s%%3A%%3A%s",
		url.PathEscape(c.UserID), url.PathEscape(c.AccessToken))
}

// ── API Response Types ──────────────────────────────────────────────

// LegacyUsage is the response from GET /api/usage (request-count billing).
type LegacyUsage struct {
	GPT4 *struct {
		NumRequests     int  `json:"numRequests"`
		MaxRequestUsage *int `json:"maxRequestUsage"`
	} `json:"gpt-4"`
	StartOfMonth string `json:"startOfMonth"`
}

// CurrentPeriodUsage is the response from POST GetCurrentPeriodUsage (USD credit billing).
type CurrentPeriodUsage struct {
	BillingCycleStart string `json:"billingCycleStart"`
	BillingCycleEnd   string `json:"billingCycleEnd"`
	PlanUsage         struct {
		Limit            *int     `json:"limit"`
		Remaining        *int     `json:"remaining"`
		Used             *int     `json:"used"`
		TotalPercentUsed *float64 `json:"totalPercentUsed"`
		AutoPercentUsed  *float64 `json:"autoPercentUsed"`
		APIPercentUsed   *float64 `json:"apiPercentUsed"`
	} `json:"planUsage"`
}

// StripeStatus is the response from GET /api/auth/stripe.
type StripeStatus struct {
	MembershipType     string  `json:"membershipType"`
	IndividualType     string  `json:"individualMembershipType"`
	SubscriptionStatus string  `json:"subscriptionStatus"`
	IsTeamMember       bool    `json:"isTeamMember"`
	IsYearlyPlan       bool    `json:"isYearlyPlan"`
	CustomerBalance    float64 `json:"customerBalance"`
}

// Snapshot is the merged result from all three endpoints.
type Snapshot struct {
	// Billing model: "request_count" or "usd_credit" or "unknown"
	BillingModel string
	// Plan info from Stripe
	PlanTier           string // free/pro/pro_plus/ultra/team/unknown
	SubscriptionStatus string
	// Legacy request-count fields
	RequestsUsed int
	RequestsMax  int
	// USD credit fields (cents)
	UsedCents        int
	LimitCents       int
	TotalPercentUsed float64
	AutoPercentUsed  float64
	APIPercentUsed   float64
	// Cycle
	CycleStart string
	CycleEnd   string
	// Partial fetch status
	LegacyOK bool
	UsageOK   bool
	StripeOK  bool
}

// UsagePct returns the overall usage percentage (0-100).
func (s *Snapshot) UsagePct() float64 {
	if s.BillingModel == "request_count" {
		if s.RequestsMax <= 0 {
			return 0
		}
		return float64(s.RequestsUsed) / float64(s.RequestsMax) * 100
	}
	return s.TotalPercentUsed
}

// ── Endpoints ───────────────────────────────────────────────────────

const (
	legacyURL  = "https://www.cursor.com/api/usage"
	stripeURL  = "https://www.cursor.com/api/auth/stripe"
	usageURL   = "https://api2.cursor.sh/aiserver.v1.DashboardService/GetCurrentPeriodUsage"
	timeout    = 15 * time.Second
)

// ── Client ──────────────────────────────────────────────────────────

// Client polls Cursor's three usage endpoints.
type Client struct {
	creds  *Credentials
	logger *slog.Logger
	http   *http.Client
}

// NewClient creates a Cursor API client.
func NewClient(creds *Credentials, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		creds:  creds,
		logger: logger,
		http:   &http.Client{Timeout: timeout},
	}
}

// FetchSnapshot polls all three endpoints and merges into a Snapshot.
func (c *Client) FetchSnapshot(ctx context.Context) (*Snapshot, error) {
	if c.creds.AccessToken == "" {
		return nil, ErrNoToken
	}

	snap := &Snapshot{BillingModel: "unknown"}

	// 1) Legacy usage (GET, Cookie auth)
	legacy, err := c.fetchLegacy(ctx)
	if err == nil {
		snap.LegacyOK = true
		if legacy.GPT4 != nil && legacy.GPT4.MaxRequestUsage != nil && *legacy.GPT4.MaxRequestUsage > 0 {
			snap.BillingModel = "request_count"
			snap.RequestsUsed = legacy.GPT4.NumRequests
			snap.RequestsMax = *legacy.GPT4.MaxRequestUsage
			snap.CycleStart = legacy.StartOfMonth
		}
	} else {
		c.logger.Debug("Cursor legacy endpoint failed", "error", err)
	}

	// 2) Current period usage (POST, Bearer auth)
	usage, err := c.fetchCurrentUsage(ctx)
	if err == nil {
		snap.UsageOK = true
		pu := usage.PlanUsage
		hasCredit := (pu.Limit != nil && *pu.Limit > 0) || (pu.TotalPercentUsed != nil)
		if hasCredit {
			snap.BillingModel = "usd_credit"
			if pu.Limit != nil {
				snap.LimitCents = *pu.Limit
			}
			if pu.Used != nil {
				snap.UsedCents = *pu.Used
			} else if pu.Limit != nil && pu.Remaining != nil {
				snap.UsedCents = *pu.Limit - *pu.Remaining
			} else if pu.Limit != nil && pu.TotalPercentUsed != nil && *pu.Limit > 0 {
				snap.UsedCents = int(float64(*pu.Limit) * *pu.TotalPercentUsed / 100)
			}
			if pu.TotalPercentUsed != nil {
				snap.TotalPercentUsed = *pu.TotalPercentUsed
			} else if snap.LimitCents > 0 {
				snap.TotalPercentUsed = float64(snap.UsedCents) / float64(snap.LimitCents) * 100
			}
			if pu.AutoPercentUsed != nil {
				snap.AutoPercentUsed = *pu.AutoPercentUsed
			}
			if pu.APIPercentUsed != nil {
				snap.APIPercentUsed = *pu.APIPercentUsed
			}
		}
		snap.CycleStart = usage.BillingCycleStart
		snap.CycleEnd = usage.BillingCycleEnd
	} else {
		c.logger.Debug("Cursor usage endpoint failed", "error", err)
	}

	// 3) Stripe status (GET, Cookie auth)
	stripe, err := c.fetchStripe(ctx)
	if err == nil {
		snap.StripeOK = true
		snap.PlanTier = detectTier(stripe)
		snap.SubscriptionStatus = stripe.SubscriptionStatus
	} else {
		c.logger.Debug("Cursor stripe endpoint failed", "error", err)
	}

	// At least one endpoint must succeed
	if !snap.LegacyOK && !snap.UsageOK && !snap.StripeOK {
		return nil, fmt.Errorf("%w: all endpoints failed", ErrUnauthorized)
	}

	c.logger.Info("Cursor snapshot fetched",
		"billing", snap.BillingModel, "plan", snap.PlanTier,
		"usagePct", snap.UsagePct(),
		"legacy", snap.LegacyOK, "usage", snap.UsageOK, "stripe", snap.StripeOK)

	return snap, nil
}

func detectTier(s *StripeStatus) string {
	if s.IsTeamMember {
		return "team"
	}
	m := strings.ToLower(s.IndividualType)
	if m == "" {
		m = strings.ToLower(s.MembershipType)
	}
	switch m {
	case "ultra":
		return "ultra"
	case "pro_plus", "pro+":
		return "pro_plus"
	case "pro":
		return "pro"
	case "free", "":
		return "free"
	default:
		return m
	}
}

// fetchLegacy calls GET /api/usage?user=<userId> with Cookie auth.
func (c *Client) fetchLegacy(ctx context.Context) (*LegacyUsage, error) {
	u := legacyURL + "?user=" + url.QueryEscape(c.creds.UserID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", c.creds.SessionCookie())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Niyantra)")

	var result LegacyUsage
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// fetchCurrentUsage calls POST GetCurrentPeriodUsage with Bearer auth.
func (c *Client) fetchCurrentUsage(ctx context.Context) (*CurrentPeriodUsage, error) {
	body := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, usageURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Niyantra)")

	var result CurrentPeriodUsage
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// fetchStripe calls GET /api/auth/stripe with Cookie auth.
func (c *Client) fetchStripe(ctx context.Context) (*StripeStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, stripeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", c.creds.SessionCookie())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Niyantra)")

	var result StripeStatus
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read: %v", ErrNetworkError, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: JSON: %v", ErrInvalidResponse, err)
	}
	return nil
}

// ── Credential Detection ────────────────────────────────────────────

var userIDRegex = regexp.MustCompile(`user_[a-zA-Z0-9]{20,}`)

// cursorDataDir returns the platform-specific Cursor config root.
func cursorDataDir() string {
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "Cursor")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Cursor")
	default:
		home, _ := os.UserHomeDir()
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			configDir = filepath.Join(home, ".config")
		}
		return filepath.Join(configDir, "Cursor")
	}
}

// DetectCredentials discovers userId + accessToken from Cursor's local files.
func DetectCredentials(logger *slog.Logger, manualToken string) (*Credentials, error) {
	if logger == nil {
		logger = slog.Default()
	}

	dataDir := cursorDataDir()

	// 1) Read accessToken from state.vscdb
	token, email := readTokenFromStateDB(dataDir, logger)
	if token == "" && manualToken != "" {
		token = manualToken
	}
	if token == "" {
		return nil, ErrNoToken
	}

	// 2) Find userId from sentry files or storage JSON files
	userID := findUserID(dataDir, logger)
	if userID == "" {
		return nil, fmt.Errorf("%w: checked sentry/scope_v3.json, session.json, storage.json", ErrNoUserID)
	}

	source := "auto"
	if manualToken != "" && token == manualToken {
		source = "manual"
	}

	logger.Debug("Cursor credentials detected",
		"userId", userID, "email", email, "source", source, "tokenLen", len(token))

	return &Credentials{
		UserID:      userID,
		AccessToken: token,
		Email:       email,
		Source:      source,
	}, nil
}

// readTokenFromStateDB reads accessToken + email from state.vscdb.
func readTokenFromStateDB(dataDir string, logger *slog.Logger) (string, string) {
	dbPath := filepath.Join(dataDir, "User", "globalStorage", "state.vscdb")
	if _, err := os.Stat(dbPath); err != nil {
		return "", ""
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro&immutable=1")
	if err != nil {
		logger.Debug("Cannot open state.vscdb", "error", err)
		return "", ""
	}
	defer db.Close()

	var token string
	_ = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken'`).Scan(&token)

	var email string
	_ = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'cursorAuth/cachedEmail'`).Scan(&email)

	return token, email
}

// findUserID searches sentry and storage files for user_xxx pattern.
func findUserID(dataDir string, logger *slog.Logger) string {
	// Priority ordered search paths (matching Tendo33's implementation)
	candidates := []string{
		filepath.Join(dataDir, "sentry", "scope_v3.json"),
		filepath.Join(dataDir, "sentry", "session.json"),
		filepath.Join(dataDir, "User", "globalStorage", "storage.json"),
		filepath.Join(dataDir, "storage.json"),
	}

	for _, path := range candidates {
		if id := extractUserIDFromFile(path, logger); id != "" {
			return id
		}
	}

	// Fallback: also check ~/.cursor/storage.json
	home, _ := os.UserHomeDir()
	if home != "" {
		dotCursor := filepath.Join(home, ".cursor", "storage.json")
		if id := extractUserIDFromFile(dotCursor, logger); id != "" {
			return id
		}
	}

	return ""
}

// extractUserIDFromFile reads a JSON file and extracts user_xxx.
func extractUserIDFromFile(path string, logger *slog.Logger) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Try structured parse first (scope_v3.json has scope.user.id)
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		// Check scope.user.id (sentry scope_v3.json)
		if scope, ok := obj["scope"].(map[string]interface{}); ok {
			if user, ok := scope["user"].(map[string]interface{}); ok {
				if id, ok := user["id"].(string); ok {
					if uid := extractUserPart(id); uid != "" {
						return uid
					}
				}
			}
		}
		// Check did field (session.json)
		if did, ok := obj["did"].(string); ok {
			if uid := extractUserPart(did); uid != "" {
				return uid
			}
		}
	}

	// Fallback: regex search for user_xxx pattern
	match := userIDRegex.FindString(string(data))
	if match != "" {
		logger.Debug("Found userId via regex", "path", path, "userId", match)
		return match
	}

	return ""
}

// extractUserPart handles OAuth IDs like "oauth|user_xxx" → "user_xxx"
func extractUserPart(s string) string {
	if strings.Contains(s, "|") {
		for _, part := range strings.Split(s, "|") {
			if strings.HasPrefix(part, "user_") {
				return part
			}
		}
	}
	if strings.HasPrefix(s, "user_") {
		return s
	}
	return ""
}
