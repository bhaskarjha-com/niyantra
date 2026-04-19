package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Sentinel errors for client operations.
var (
	ErrProcessNotFound  = errors.New("antigravity: language server process not found")
	ErrPortNotFound     = errors.New("antigravity: no listening port found")
	ErrConnectionFailed = errors.New("antigravity: connection failed")
	ErrInvalidResponse  = errors.New("antigravity: invalid response")
	ErrNotAuthenticated = errors.New("antigravity: not authenticated")
)

// lsEndpoint is the Connect RPC service path for quota retrieval.
const lsEndpoint = "/exa.language_server_pb.LanguageServerService/GetUserStatus"

// processInfo holds auto-detected process metadata.
type processInfo struct {
	PID                 int
	CSRFToken           string
	ExtensionServerPort int
	CommandLine         string
}

// connection holds a verified language server endpoint.
type connection struct {
	BaseURL   string
	CSRFToken string
	Port      int
	Protocol  string
}

// Client communicates with the local Antigravity language server.
type Client struct {
	transport *http.Client
	conn      *connection
	logger    *slog.Logger
}

// New returns a Client ready to detect the language server.
func New(logger *slog.Logger) *Client {
	return &Client{
		transport: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxConnsPerHost:       2,
				ResponseHeaderTimeout: 12 * time.Second,
				IdleConnTimeout:       45 * time.Second,
				TLSHandshakeTimeout:   6 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // language server uses self-signed certs
				},
			},
		},
		logger: logger,
	}
}

// Detect locates and verifies a connection to the language server.
// Subsequent calls return immediately if a connection is cached.
func (c *Client) Detect(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}

	c.logger.Debug("searching for Antigravity language server")

	proc, err := c.detectProcess(ctx)
	if err != nil {
		return err
	}

	c.logger.Debug("language server process located",
		"pid", proc.PID,
		"hasToken", proc.CSRFToken != "",
	)

	ports, err := c.discoverPorts(ctx, proc.PID)
	if err != nil || len(ports) == 0 {
		return ErrPortNotFound
	}

	c.logger.Debug("candidate ports discovered", "count", len(ports))

	verified, err := c.verifyEndpoint(ctx, ports, proc.CSRFToken)
	if err != nil {
		return err
	}

	c.conn = verified
	c.logger.Info("language server connection established",
		"port", verified.Port,
		"tls", verified.Protocol == "https",
	)

	return nil
}

// FetchQuotas retrieves the current quota status from the language server.
func (c *Client) FetchQuotas(ctx context.Context) (*UserStatusResponse, error) {
	if err := c.Detect(ctx); err != nil {
		return nil, err
	}

	url := c.conn.BaseURL + lsEndpoint
	payload := `{"metadata":{"ideName":"antigravity","extensionName":"antigravity","locale":"en"}}`

	fetchCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("antigravity: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	if c.conn.CSRFToken != "" {
		req.Header.Set("X-Codeium-Csrf-Token", c.conn.CSRFToken)
	}

	resp, err := c.transport.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		c.conn = nil // stale connection — force re-detect
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.conn = nil
		return nil, fmt.Errorf("antigravity: server returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // cap at 64 KiB
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrInvalidResponse, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: empty body", ErrInvalidResponse)
	}

	var out UserStatusResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	if out.UserStatus == nil {
		if out.Message != "" {
			return nil, fmt.Errorf("%w: %s", ErrNotAuthenticated, out.Message)
		}
		return nil, ErrNotAuthenticated
	}

	return &out, nil
}

// Reset forces the next call to Detect to re-discover the language server.
func (c *Client) Reset() {
	c.conn = nil
}

// IsConnected reports whether a cached connection exists.
func (c *Client) IsConnected() bool {
	return c.conn != nil
}

// ConnInfo exposes connection details for diagnostics.
type ConnInfo struct {
	BaseURL   string
	CSRFToken string
	Port      int
	Protocol  string
}

// ConnectionInfo returns cached connection details, or nil.
func (c *Client) ConnectionInfo() *ConnInfo {
	if c.conn == nil {
		return nil
	}
	return &ConnInfo{
		BaseURL:   c.conn.BaseURL,
		CSRFToken: c.conn.CSRFToken,
		Port:      c.conn.Port,
		Protocol:  c.conn.Protocol,
	}
}
