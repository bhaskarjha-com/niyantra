package codex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Errors ──────────────────────────────────────────────────────────

var (
	ErrUnauthorized       = errors.New("codex: unauthorized")
	ErrForbidden          = errors.New("codex: forbidden")
	ErrServerError        = errors.New("codex: server error")
	ErrNetworkError       = errors.New("codex: network error")
	ErrInvalidResponse    = errors.New("codex: invalid response")
	ErrRefreshFailed      = errors.New("codex: oauth token refresh failed")
	ErrRefreshTokenReused = errors.New("codex: refresh token already used (re-authenticate via 'codex auth')")
	ErrNotInstalled       = errors.New("codex: auth.json not found")
)

// ── Credentials ─────────────────────────────────────────────────────

// Credentials contains parsed Codex auth state from ~/.codex/auth.json.
type Credentials struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	APIKey       string
	AccountID    string
	UserID       string
	Email        string
	Name         string
	Picture      string
	ExpiresAt    time.Time
	ExpiresIn    time.Duration
}

// IsExpiringSoon returns true if the token expires within the given duration.
func (c *Credentials) IsExpiringSoon(threshold time.Duration) bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return c.ExpiresIn < threshold
}

// IsExpired returns true if token has already expired.
func (c *Credentials) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return c.ExpiresIn <= 0
}

// authFile is the on-disk structure of ~/.codex/auth.json.
type authFile struct {
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
	Tokens       struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
}

// DetectCredentials loads Codex credentials from CODEX_HOME/auth.json or ~/.codex/auth.json.
func DetectCredentials(logger *slog.Logger) (*Credentials, error) {
	if logger == nil {
		logger = slog.Default()
	}

	authPath := authFilePath()
	if authPath == "" {
		return nil, ErrNotInstalled
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotInstalled
		}
		return nil, fmt.Errorf("codex: reading auth file: %w", err)
	}

	var auth authFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("codex: parsing auth file: %w", err)
	}

	idToken := strings.TrimSpace(auth.Tokens.IDToken)
	expiresAt := parseIDTokenExpiry(idToken)
	var expiresIn time.Duration
	if !expiresAt.IsZero() {
		expiresIn = time.Until(expiresAt)
	}

	creds := &Credentials{
		AccessToken:  strings.TrimSpace(auth.Tokens.AccessToken),
		RefreshToken: strings.TrimSpace(auth.Tokens.RefreshToken),
		IDToken:      idToken,
		APIKey:       strings.TrimSpace(auth.OpenAIAPIKey),
		AccountID:    strings.TrimSpace(auth.Tokens.AccountID),
		UserID:       parseIDTokenUserID(idToken),
		Email:        parseIDTokenEmail(idToken),
		Name:         parseIDTokenName(idToken),
		Picture:      parseIDTokenPicture(idToken),
		ExpiresAt:    expiresAt,
		ExpiresIn:    expiresIn,
	}

	if creds.AccessToken == "" && creds.APIKey == "" {
		return nil, fmt.Errorf("codex: auth file has no usable token")
	}

	logger.Debug("Codex credentials loaded",
		"path", authPath,
		"has_refresh_token", creds.RefreshToken != "",
		"account_id", creds.AccountID,
		"email", creds.Email,
		"name", creds.Name,
		"user_id", creds.UserID,
		"has_picture", creds.Picture != "")

	return creds, nil
}

// authFilePath resolves the Codex auth.json location.
func authFilePath() string {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return filepath.Join(codexHome, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".codex", "auth.json")
}

// ── OAuth Token Refresh ─────────────────────────────────────────────

const (
	oauthClientID = "app_EMoamEEZ73f0CkXaXp7hrann" // Codex CLI OAuth client ID
	oauthTokenURL = "https://auth.openai.com/oauth/token"
	oauthScope    = "openid profile email offline_access"
)

// OAuthTokenResponse from OpenAI's OAuth token endpoint.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// RefreshToken exchanges a refresh token for a new access token.
// IMPORTANT: OpenAI uses one-time-use refresh tokens (rotation).
// The new refresh token MUST be saved immediately after success.
func RefreshToken(ctx context.Context, refreshToken string) (*OAuthTokenResponse, error) {
	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("refresh_token", refreshToken)
	formData.Set("client_id", oauthClientID)
	formData.Set("scope", oauthScope)

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, oauthTokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("codex oauth: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "niyantra/1.0")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("codex oauth: network error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("codex oauth: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			if errResp.Error == "invalid_grant" && strings.Contains(errResp.Description, "reused") {
				return nil, ErrRefreshTokenReused
			}
			return nil, fmt.Errorf("%w: %s - %s", ErrRefreshFailed, errResp.Error, errResp.Description)
		}
		return nil, fmt.Errorf("%w: HTTP %d", ErrRefreshFailed, resp.StatusCode)
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("codex oauth: parse response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("%w: empty access token", ErrRefreshFailed)
	}

	return &tokenResp, nil
}

// WriteCredentials saves refreshed OAuth tokens to auth.json atomically.
// Creates backup before modifying, preserves existing fields.
func WriteCredentials(accessToken, refreshToken, idToken string) error {
	authPath := authFilePath()
	if authPath == "" {
		return os.ErrNotExist
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return err
		}
	}

	// S5: Backup before modifying — abort if backup fails to avoid data loss
	if len(data) > 2 {
		if err := os.WriteFile(authPath+".bak", data, 0600); err != nil {
			return fmt.Errorf("codex: backup auth file before modification: %w", err)
		}
	}

	var rawAuth map[string]interface{}
	if json.Unmarshal(data, &rawAuth) != nil {
		rawAuth = make(map[string]interface{})
	}

	tokens, ok := rawAuth["tokens"].(map[string]interface{})
	if !ok {
		tokens = make(map[string]interface{})
		rawAuth["tokens"] = tokens
	}

	tokens["access_token"] = accessToken
	tokens["refresh_token"] = refreshToken
	if idToken != "" {
		tokens["id_token"] = idToken
	}

	newData, err := json.MarshalIndent(rawAuth, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(authPath), 0700); err != nil {
		return err
	}

	tempPath := authPath + ".tmp"
	if err := os.WriteFile(tempPath, newData, 0600); err != nil {
		return err
	}
	return os.Rename(tempPath, authPath)
}

// ── JWT Parsing ─────────────────────────────────────────────────────

// parseIDTokenExpiry extracts the exp claim from a JWT id_token.
func parseIDTokenExpiry(idToken string) time.Time {
	claims := parseJWTPayload(idToken)
	if claims == nil {
		return time.Time{}
	}
	if exp, ok := claims["exp"].(float64); ok && exp > 0 {
		return time.Unix(int64(exp), 0)
	}
	return time.Time{}
}

// parseIDTokenEmail extracts the email claim from a JWT id_token.
// OpenAI's OIDC id_token includes a standard "email" claim.
func parseIDTokenEmail(idToken string) string {
	claims := parseJWTPayload(idToken)
	if claims == nil {
		return ""
	}
	if email, ok := claims["email"].(string); ok {
		return strings.TrimSpace(email)
	}
	return ""
}

// parseIDTokenName extracts the name claim from a JWT id_token.
func parseIDTokenName(idToken string) string {
	claims := parseJWTPayload(idToken)
	if claims == nil {
		return ""
	}
	if name, ok := claims["name"].(string); ok {
		return strings.TrimSpace(name)
	}
	return ""
}

// parseIDTokenPicture extracts the picture claim from a JWT id_token.
func parseIDTokenPicture(idToken string) string {
	claims := parseJWTPayload(idToken)
	if claims == nil {
		return ""
	}
	if pic, ok := claims["picture"].(string); ok {
		return strings.TrimSpace(pic)
	}
	return ""
}

// parseIDTokenUserID extracts chatgpt_user_id from a JWT id_token.
func parseIDTokenUserID(idToken string) string {
	claims := parseJWTPayload(idToken)
	if claims == nil {
		return ""
	}
	authClaims, ok := claims["https://api.openai.com/auth"].(map[string]interface{})
	if !ok {
		return ""
	}
	if uid, ok := authClaims["chatgpt_user_id"].(string); ok {
		return strings.TrimSpace(uid)
	}
	if uid, ok := authClaims["user_id"].(string); ok {
		return strings.TrimSpace(uid)
	}
	return ""
}

func parseJWTPayload(token string) map[string]interface{} {
	if token == "" {
		return nil
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims map[string]interface{}
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}

// ── Usage API Client ────────────────────────────────────────────────

const defaultBaseURL = "https://chatgpt.com/backend-api/wham/usage"

// Client fetches Codex usage data via the internal ChatGPT API.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	fallbackURL string
	token       string
	accountID   string
	logger      *slog.Logger
}

// NewClient creates a Codex API client.
func NewClient(token, accountID string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          1,
				MaxIdleConnsPerHost:   1,
				ResponseHeaderTimeout: 10 * time.Second,
				IdleConnTimeout:       10 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ForceAttemptHTTP2:     true,
			},
		},
		baseURL:   defaultBaseURL,
		token:     token,
		accountID: accountID,
		logger:    logger,
	}
}

// SetToken updates the bearer token.
func (c *Client) SetToken(token string) { c.token = token }

// UsageResponse is the normalized Codex usage payload.
type UsageResponse struct {
	PlanType       string   `json:"plan_type"`
	Quotas         []Quota  `json:"quotas"`
	CreditsBalance *float64 `json:"credits_balance,omitempty"`
}

// Quota represents one normalized Codex quota window.
type Quota struct {
	Name        string     `json:"name"`        // "five_hour", "seven_day", "code_review"
	Utilization float64    `json:"utilization"` // 0-100
	ResetsAt    *time.Time `json:"resets_at"`   // when window resets
	Status      string     `json:"status"`      // healthy/warning/danger/critical
}

// rawUsageResponse matches the actual API response shape.
type rawUsageResponse struct {
	PlanType            string       `json:"plan_type"`
	RateLimit           rawRateLimit `json:"rate_limit"`
	CodeReviewRateLimit rawRateLimit `json:"code_review_rate_limit,omitempty"`
	Credits             *struct {
		Balance json.RawMessage `json:"balance,omitempty"`
	} `json:"credits,omitempty"`
}

type rawRateLimit struct {
	PrimaryWindow   *rawWindow `json:"primary_window"`
	SecondaryWindow *rawWindow `json:"secondary_window"`
}

type rawWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	ResetAtUnix        int64   `json:"reset_at"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
}

// FetchUsage calls the Codex usage API and returns normalized data.
func (c *Client) FetchUsage(ctx context.Context) (*UsageResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	usageURL := c.baseURL
	if c.fallbackURL != "" {
		usageURL = c.fallbackURL
	}

	resp, err := c.doRequest(reqCtx, usageURL)
	if err != nil {
		return nil, err
	}

	// Try fallback URL on 404
	if resp.StatusCode == http.StatusNotFound {
		if fallback := buildFallbackURL(usageURL); fallback != "" {
			resp.Body.Close()
			c.fallbackURL = fallback
			resp, err = c.doRequest(reqCtx, fallback)
			if err != nil {
				return nil, err
			}
		}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		return nil, ErrForbidden
	case resp.StatusCode >= 500:
		return nil, ErrServerError
	default:
		return nil, fmt.Errorf("codex: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("%w: reading body: %v", ErrInvalidResponse, err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("%w: empty response", ErrInvalidResponse)
	}

	var raw rawUsageResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	return c.normalize(&raw), nil
}

func (c *Client) doRequest(ctx context.Context, usageURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("codex: creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "niyantra/1.0")
	if c.accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", c.accountID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	return resp, nil
}

func (c *Client) normalize(raw *rawUsageResponse) *UsageResponse {
	resp := &UsageResponse{PlanType: raw.PlanType}

	// Parse credits balance (can be number or string)
	if raw.Credits != nil && raw.Credits.Balance != nil {
		var bal float64
		if json.Unmarshal(raw.Credits.Balance, &bal) == nil {
			resp.CreditsBalance = &bal
		}
	}

	// Primary window → determine name from context
	if raw.RateLimit.PrimaryWindow != nil {
		name := "five_hour"
		if raw.RateLimit.SecondaryWindow == nil {
			if strings.EqualFold(strings.TrimSpace(raw.PlanType), "free") ||
				raw.RateLimit.PrimaryWindow.LimitWindowSeconds >= 7*24*60*60 {
				name = "seven_day"
			}
		}
		resp.Quotas = append(resp.Quotas, quotaFromWindow(name, raw.RateLimit.PrimaryWindow))
	}
	if raw.RateLimit.SecondaryWindow != nil {
		resp.Quotas = append(resp.Quotas, quotaFromWindow("seven_day", raw.RateLimit.SecondaryWindow))
	}
	if raw.CodeReviewRateLimit.PrimaryWindow != nil {
		resp.Quotas = append(resp.Quotas, quotaFromWindow("code_review", raw.CodeReviewRateLimit.PrimaryWindow))
	}

	return resp
}

func quotaFromWindow(name string, w *rawWindow) Quota {
	q := Quota{
		Name:        name,
		Utilization: w.UsedPercent,
		Status:      statusFromUtilization(w.UsedPercent),
	}
	if w.ResetAtUnix > 0 {
		t := time.Unix(w.ResetAtUnix, 0).UTC()
		q.ResetsAt = &t
	}
	return q
}

func statusFromUtilization(u float64) string {
	switch {
	case u >= 95:
		return "critical"
	case u >= 80:
		return "danger"
	case u >= 50:
		return "warning"
	default:
		return "healthy"
	}
}

func buildFallbackURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch {
	case strings.Contains(u.Path, "/api/codex/usage"):
		u.Path = strings.Replace(u.Path, "/api/codex/usage", "/backend-api/wham/usage", 1)
	case strings.Contains(u.Path, "/backend-api/wham/usage"):
		u.Path = strings.Replace(u.Path, "/backend-api/wham/usage", "/api/codex/usage", 1)
	default:
		return ""
	}
	u.RawQuery = ""
	return u.String()
}
