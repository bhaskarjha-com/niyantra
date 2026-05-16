// Package mcpserver implements a Model Context Protocol (MCP) server
// that exposes Niyantra's quota intelligence to AI coding agents.
//
// The server supports two transports:
//   - stdio (JSON-RPC 2.0) for local AI agent integration via `niyantra mcp`
//   - Streamable HTTP for remote clients via the `/mcp` endpoint on the web dashboard
//
// It provides 12 tools for querying quota status, model availability, usage
// intelligence, budget forecasts, model recommendations, spending analysis,
// switch advice, Codex status, quota time-to-exhaustion, token usage
// analytics, git commit cost correlation, and plugin data.
//
// Tool handlers are organized into domain files:
//
//	tools_quota.go     — quota_status, model_availability, usage_intelligence, best_model
//	tools_ops.go       — budget_forecast, analyze_spending, switch_recommendation, codex_status
//	tools_forecast.go  — quota_forecast, cost estimation helpers
//	tools_analytics.go — token_usage_stats, git_commit_costs, formatting helpers
package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tracker"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer wraps the MCP server with Niyantra-specific tools.
type MCPServer struct {
	store   *store.Store
	tracker *tracker.Tracker
	logger  *slog.Logger
	server  *mcp.Server
}

// New creates an MCPServer with all tools registered.
func New(s *store.Store, t *tracker.Tracker, logger *slog.Logger, version string) *MCPServer {
	if version == "" {
		version = "dev"
	}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "niyantra",
		Version: version,
	}, nil)

	m := &MCPServer{
		store:   s,
		tracker: t,
		logger:  logger,
		server:  srv,
	}

	// Register all tools
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "quota_status",
		Description: "Get quota status for all tracked Antigravity/Windsurf accounts. Returns per-account readiness with quota group percentages and reset timers.",
	}, m.handleQuotaStatus)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "model_availability",
		Description: "Check availability of a specific model or quota group. Accepts a model name (e.g. 'Claude Sonnet', 'Gemini Pro', 'GPT') and returns remaining quota, reset time, and usage rate if available.",
	}, m.handleModelAvailability)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "usage_intelligence",
		Description: "Get detailed usage intelligence for all tracked models including consumption rates, projected usage at reset, projected exhaustion times, and cycle history. Requires 30+ minutes of tracking data for rate calculations.",
	}, m.handleUsageIntelligence)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "budget_forecast",
		Description: "Get budget burn rate forecast including daily burn rate, projected monthly spend, and whether spending is on track relative to the configured monthly budget.",
	}, m.handleBudgetForecast)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "best_model",
		Description: "Recommend the best model to use in a quota group based on remaining quota, consumption rate, and time until reset. Groups: 'claude_gpt' (Claude + GPT models), 'gemini_pro' (Gemini Pro), 'gemini_flash' (Gemini Flash).",
	}, m.handleBestModel)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "analyze_spending",
		Description: "Analyze AI subscription spending patterns. Returns category breakdown, monthly/annual totals, savings opportunities (annual billing, unused subs, category overlap), and budget status.",
	}, m.handleAnalyzeSpending)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "switch_recommendation",
		Description: "Get a recommendation on which Antigravity/Windsurf account to use right now. Analyzes remaining quota, burn rate, and reset timers across all accounts. Returns 'switch' (use a different account), 'stay' (current is optimal), or 'wait' (all exhausted, reset imminent) with scoring breakdown.",
	}, m.handleSwitchRecommendation)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "codex_status",
		Description: "Get Codex/ChatGPT quota detection state. Shows if Codex CLI is installed, current plan, token expiry, and latest usage snapshot with 5-hour and 7-day window utilization.",
	}, m.handleCodexStatus)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "quota_forecast",
		Description: "Get time-to-exhaustion (TTX) forecasts for all tracked quota groups across all providers. Uses sliding-window rate calculations from recent snapshot history (last 60 min) for accurate burn rate predictions. Returns per-group TTX estimates with severity levels (safe/caution/warning/critical) and confidence indicators. Covers Antigravity model groups, Claude Code, and Codex providers.",
	}, m.handleQuotaForecast)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "token_usage_stats",
		Description: "Get unified token usage analytics across all AI coding providers. Returns total token counts (input/output/cache), estimated costs, per-model breakdowns, daily trends, and KPIs (cache hit rate, avg tokens/day, peak day). Supports time range filtering (default 30 days). Primary data source is Claude Code JSONL sessions with full per-turn granularity.",
	}, m.handleTokenUsageStats)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "git_commit_costs",
		Description: "Correlate git commits with actual AI token consumption. For each recent commit, finds overlapping Claude Code sessions within a 30-minute window and reports real token usage and cost — not estimated from diffs. Returns per-commit costs, branch-level aggregation, and totals. Provide a repo path or defaults to the current working directory.",
	}, m.handleGitCommitCosts)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "copilot_status",
		Description: "Get GitHub Copilot detection state and usage. Shows if a PAT is configured, current plan (Pro/Pro+/Free/Business/Enterprise), premium interaction usage percentage, chat usage percentage, and latest snapshot. Requires a GitHub PAT with read:user scope.",
	}, m.handleCopilotStatus)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "plugin_status",
		Description: "Get the latest data from all installed external plugins (F18 Plugin System). Optionally filter by plugin_id. Returns each plugin's provider name, label, usage percentage, usage display string, plan type, and last capture timestamp. Plugins are external scripts in ~/.niyantra/plugins/ that track custom AI services.",
	}, m.handlePluginStatus)

	return m
}

// Run starts the MCP server over stdio, blocking until the client disconnects.
func (m *MCPServer) Run(ctx context.Context) error {
	m.logger.Info("MCP server starting over stdio")
	return m.server.Run(ctx, &mcp.StdioTransport{})
}

// HTTPHandler returns an http.Handler for the Streamable HTTP transport.
// Mount this on your HTTP server (e.g. at /mcp) to allow remote MCP clients
// like Claude Desktop, CI/CD pipelines, or cross-machine agents to connect.
func (m *MCPServer) HTTPHandler() http.Handler {
	m.logger.Info("MCP Streamable HTTP handler initialized")
	return mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return m.server
	}, nil)
}

// formatDuration formats a duration as a human-readable string like "2h30m".
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
