package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// verifyEndpoint tries each candidate port to locate the Connect RPC service.
func (c *Client) verifyEndpoint(ctx context.Context, ports []int, csrfToken string) (*connection, error) {
	for _, port := range ports {
		// Prefer TLS — the language server normally binds with self-signed certs
		if conn := c.tryPort(ctx, port, "https", csrfToken); conn != nil {
			return conn, nil
		}
		if conn := c.tryPort(ctx, port, "http", csrfToken); conn != nil {
			return conn, nil
		}
	}
	return nil, ErrConnectionFailed
}

// tryPort sends a lightweight RPC probe to confirm the port is a Connect endpoint.
func (c *Client) tryPort(ctx context.Context, port int, scheme, csrfToken string) *connection {
	base := fmt.Sprintf("%s://127.0.0.1:%d", scheme, port)
	probe := base + "/exa.language_server_pb.LanguageServerService/GetUnleashData"

	probeCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probe, strings.NewReader(`{"wrapper_data":{}}`))
	if err != nil {
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	if csrfToken != "" {
		req.Header.Set("X-Codeium-Csrf-Token", csrfToken)
	}

	resp, err := c.transport.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	// A valid Connect RPC service replies 200 (success) or 401 (needs auth).
	// Anything else (404, connection refused) means wrong port.
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		return &connection{
			BaseURL:   base,
			CSRFToken: csrfToken,
			Port:      port,
			Protocol:  scheme,
		}
	}

	return nil
}
