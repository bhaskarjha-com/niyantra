package mcpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testLogger returns a silent logger for tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestHTTPHandlerResponds verifies the Streamable HTTP handler returns
// a valid MCP JSON-RPC response (or acceptable error) for an initialize request.
func TestHTTPHandlerResponds(t *testing.T) {
	// Create a minimal MCPServer without a real store/tracker.
	// The handler should still respond to the MCP initialize method.
	m := New(nil, nil, testLogger(), "test")
	handler := m.HTTPHandler()

	// Send a valid JSON-RPC initialize request
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// The SDK should respond — either 200 with JSON-RPC result, or a protocol-level error.
	// Any response other than a panic or 5xx means the handler is wired correctly.
	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected non-5xx response from MCP handler, got %d: %s", resp.StatusCode, string(respBody))
	}
}

// TestHTTPHandlerRejectsGETWithoutSession verifies GET without session returns
// an appropriate error (sessions require POST initialization first).
func TestHTTPHandlerRejectsGETWithoutSession(t *testing.T) {
	m := New(nil, nil, testLogger(), "test")
	handler := m.HTTPHandler()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// GET without a valid session should be rejected
	if rec.Code == http.StatusOK {
		t.Error("expected non-200 for GET without session, got 200")
	}
}
