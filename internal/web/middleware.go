package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// basicAuth wraps a handler with HTTP basic authentication.
func (s *Server) basicAuth(next http.Handler) http.Handler {
	parts := strings.SplitN(s.auth, ":", 2)
	if len(parts) != 2 {
		return next
	}
	user, pass := parts[0], parts[1]

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check only
		// /healthz: container probes need unauthenticated access
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Niyantra"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityMiddleware enforces CORS and Content-Type policies.
func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS: only allow localhost origin matching our port
		allowedOrigin := fmt.Sprintf("http://localhost:%d", s.port)
		allowedOrigin2 := fmt.Sprintf("http://127.0.0.1:%d", s.port)
		origin := r.Header.Get("Origin")

		// MCP endpoint: enforce strict Origin check to prevent cross-site exfiltration.
		// Requests without Origin header are allowed (CLI tools, MCP SDK clients).
		if r.URL.Path == "/mcp" && origin != "" &&
			origin != allowedOrigin && origin != allowedOrigin2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "cross-origin MCP requests are not allowed",
			})
			return
		}

		if origin == allowedOrigin || origin == allowedOrigin2 {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Enforce Content-Type: application/json on mutation endpoints
		// Skip for /mcp — the MCP SDK handler manages its own content negotiation
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				ct := r.Header.Get("Content-Type")
				// Allow empty content-type for DELETE and requests with no body
				if ct != "" && !strings.HasPrefix(ct, "application/json") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnsupportedMediaType)
					json.NewEncoder(w).Encode(map[string]string{"error": "Content-Type must be application/json"})
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
