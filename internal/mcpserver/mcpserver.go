// Package mcpserver implements a Model Context Protocol (MCP) server
// that exposes Niyantra's quota intelligence to AI coding agents.
//
// The server communicates over stdio (JSON-RPC 2.0) and provides
// 11 tools for querying quota status, model availability, usage
// intelligence, budget forecasts, model recommendations, spending
// analysis, switch advice, Codex status, quota time-to-exhaustion,
// token usage analytics, and git commit cost correlation.
package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/advisor"
	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/codex"
	"github.com/bhaskarjha-com/niyantra/internal/costtrack"
	"github.com/bhaskarjha-com/niyantra/internal/forecast"
	"github.com/bhaskarjha-com/niyantra/internal/gitcorr"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tokenusage"
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

	return m
}

// Run starts the MCP server over stdio, blocking until the client disconnects.
func (m *MCPServer) Run(ctx context.Context) error {
	m.logger.Info("MCP server starting over stdio")
	return m.server.Run(ctx, &mcp.StdioTransport{})
}

// ──────────────────────────────────────────
//  Input/Output types
// ──────────────────────────────────────────

// EmptyInput is used for tools that take no arguments.
type EmptyInput struct{}

// ModelInput is the input for model_availability.
type ModelInput struct {
	Model string `json:"model" jsonschema:"the model name or keyword to search for, e.g. 'Claude Sonnet', 'Gemini Pro', 'GPT'"`
}

// GroupInput is the input for best_model.
type GroupInput struct {
	Group string `json:"group" jsonschema:"the quota group key: 'claude_gpt', 'gemini_pro', or 'gemini_flash'"`
}

// QuotaStatusOutput is the output of quota_status.
type QuotaStatusOutput struct {
	Accounts      []AccountSummary `json:"accounts"`
	AccountCount  int              `json:"accountCount"`
	SnapshotCount int              `json:"snapshotCount"`
}

// AccountSummary is a single account in quota_status output.
type AccountSummary struct {
	Email         string         `json:"email"`
	Plan          string         `json:"plan"`
	IsReady       bool           `json:"isReady"`
	Staleness     string         `json:"staleness"`
	AICredits     float64        `json:"aiCredits,omitempty"`
	EstimatedCost string         `json:"estimatedCost,omitempty"`
	Groups        []GroupSummary `json:"groups"`
}

// GroupSummary is a quota group within an account.
type GroupSummary struct {
	Name        string `json:"name"`
	GroupKey    string `json:"groupKey"`
	Remaining   int    `json:"remainingPercent"`
	IsExhausted bool   `json:"isExhausted"`
	ResetIn     string `json:"resetIn,omitempty"`
}

// ModelAvailOutput is the output of model_availability.
type ModelAvailOutput struct {
	Found     bool   `json:"found"`
	ModelID   string `json:"modelId,omitempty"`
	Label     string `json:"label,omitempty"`
	Group     string `json:"group,omitempty"`
	Available bool   `json:"available"`
	Remaining int    `json:"remainingPercent"`
	ResetIn   string `json:"resetIn,omitempty"`
	Rate      string `json:"rate,omitempty"`
	Message   string `json:"message"`
}

// IntelligenceOutput wraps usage intelligence data.
type IntelligenceOutput struct {
	Models  []ModelIntel `json:"models"`
	Message string       `json:"message"`
}

// ModelIntel is per-model intelligence data.
type ModelIntel struct {
	ModelID             string `json:"modelId"`
	Label               string `json:"label"`
	Group               string `json:"group"`
	RemainingPercent    int    `json:"remainingPercent"`
	IsExhausted         bool   `json:"isExhausted"`
	ResetIn             string `json:"resetIn,omitempty"`
	CurrentRate         string `json:"currentRate,omitempty"`
	ProjectedUsage      string `json:"projectedUsage,omitempty"`
	ProjectedExhaustion string `json:"projectedExhaustion,omitempty"`
	HasIntelligence     bool   `json:"hasIntelligence"`
	CompletedCycles     int    `json:"completedCycles"`
	CycleAge            string `json:"cycleAge,omitempty"`
}

// BudgetOutput is the output of budget_forecast.
type BudgetOutput struct {
	HasBudget        bool    `json:"hasBudget"`
	MonthlyBudget    float64 `json:"monthlyBudget,omitempty"`
	CurrentSpend     float64 `json:"currentSpend,omitempty"`
	ProjectedSpend   float64 `json:"projectedMonthlySpend,omitempty"`
	BurnRate         float64 `json:"burnRate,omitempty"`
	OnTrack          bool    `json:"onTrack"`
	DaysUntilExhaust *int    `json:"daysUntilBudgetExhausted,omitempty"`
	Message          string  `json:"message"`
}

// BestModelOutput is the output of best_model.
type BestModelOutput struct {
	Found        bool               `json:"found"`
	Recommended  string             `json:"recommended,omitempty"`
	Reason       string             `json:"reason"`
	Alternatives []ModelAlternative `json:"alternatives,omitempty"`
}

// ModelAlternative is a single model option in best_model output.
type ModelAlternative struct {
	Label     string `json:"label"`
	Remaining int    `json:"remainingPercent"`
	Rate      string `json:"rate,omitempty"`
	ResetIn   string `json:"resetIn,omitempty"`
}

// SpendingOutput is the output of analyze_spending.
type SpendingOutput struct {
	TotalMonthly      float64         `json:"totalMonthly"`
	TotalAnnual       float64         `json:"totalAnnual"`
	Currency          string          `json:"currency"`
	SubscriptionCount int             `json:"subscriptionCount"`
	Categories        []CategorySpend `json:"categories"`
	Insights          []store.Insight `json:"insights"`
	BudgetStatus      *BudgetStatus   `json:"budgetStatus,omitempty"`
	Message           string          `json:"message"`
}

// CategorySpend is a spending breakdown by category.
type CategorySpend struct {
	Name    string  `json:"name"`
	Monthly float64 `json:"monthly"`
	Count   int     `json:"count"`
}

// BudgetStatus summarizes budget utilization.
type BudgetStatus struct {
	MonthlyBudget float64 `json:"monthlyBudget"`
	CurrentSpend  float64 `json:"currentSpend"`
	PercentUsed   float64 `json:"percentUsed"`
	OnTrack       bool    `json:"onTrack"`
}

// SwitchOutput is the output of switch_recommendation.
type SwitchOutput struct {
	Action       string                 `json:"action"`
	BestAccount  *advisor.AccountScore  `json:"bestAccount,omitempty"`
	Alternatives []advisor.AccountScore `json:"alternatives,omitempty"`
	Reason       string                 `json:"reason"`
	Message      string                 `json:"message"`
}

// CodexStatusOutput is the output of codex_status.
type CodexStatusOutput struct {
	Installed      bool                 `json:"installed"`
	CaptureEnabled bool                 `json:"captureEnabled"`
	AccountID      string               `json:"accountId,omitempty"`
	TokenExpired   bool                 `json:"tokenExpired,omitempty"`
	TokenExpiresIn string               `json:"tokenExpiresIn,omitempty"`
	Snapshot       *store.CodexSnapshot `json:"snapshot,omitempty"`
	Message        string               `json:"message"`
}

// ──────────────────────────────────────────
//  Tool handlers
// ──────────────────────────────────────────

func (m *MCPServer) handleQuotaStatus(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, QuotaStatusOutput, error) {
	snapshots, err := m.store.LatestPerAccount()
	if err != nil {
		return nil, QuotaStatusOutput{}, fmt.Errorf("failed to load snapshots: %w", err)
	}

	accounts := readiness.Calculate(snapshots, 0.0)
	out := QuotaStatusOutput{
		AccountCount:  m.store.AccountCount(),
		SnapshotCount: m.store.SnapshotCount(),
	}

	for _, acc := range accounts {
		a := AccountSummary{
			Email:     acc.Email,
			Plan:      acc.PlanName,
			IsReady:   acc.IsReady,
			Staleness: acc.StalenessLabel,
		}
		if len(acc.AICredits) > 0 {
			a.AICredits = acc.AICredits[0].CreditAmount
		}
		for _, g := range acc.Groups {
			gs := GroupSummary{
				Name:        g.DisplayName,
				GroupKey:    g.GroupKey,
				Remaining:   int(math.Round(g.RemainingPercent)),
				IsExhausted: g.IsExhausted,
			}
			if g.TimeUntilResetSec > 0 {
				gs.ResetIn = formatDuration(time.Duration(g.TimeUntilResetSec) * time.Second)
			}
			a.Groups = append(a.Groups, gs)
		}
		out.Accounts = append(out.Accounts, a)
	}

	// F8: Compute estimated costs for each account
	costByAccount := m.computeMCPCosts(snapshots)
	if costByAccount != nil {
		for i := range out.Accounts {
			for _, snap := range snapshots {
				if snap != nil && snap.Email == out.Accounts[i].Email {
					if est, ok := costByAccount[snap.AccountID]; ok {
						out.Accounts[i].EstimatedCost = est.TotalLabel
					}
					break
				}
			}
		}
	}

	return nil, out, nil
}

func (m *MCPServer) handleModelAvailability(_ context.Context, _ *mcp.CallToolRequest, input ModelInput) (*mcp.CallToolResult, ModelAvailOutput, error) {
	query := strings.ToLower(input.Model)
	if query == "" {
		return nil, ModelAvailOutput{Message: "Please provide a model name to search for."}, nil
	}

	snapshots, err := m.store.LatestPerAccount()
	if err != nil {
		return nil, ModelAvailOutput{}, fmt.Errorf("failed to load snapshots: %w", err)
	}

	// Search across all accounts for matching model
	for _, snap := range snapshots {
		for _, model := range snap.Models {
			label := strings.ToLower(model.Label)
			id := strings.ToLower(model.ModelID)
			group := client.GroupForModel(model.ModelID, model.Label)
			if strings.Contains(label, query) || strings.Contains(id, query) {
				pct := int(math.Round(model.RemainingFraction * 100))
				out := ModelAvailOutput{
					Found:     true,
					ModelID:   model.ModelID,
					Label:     model.Label,
					Group:     group,
					Available: !model.IsExhausted && pct > 0,
					Remaining: pct,
					Message:   fmt.Sprintf("%s: %d%% remaining", model.Label, pct),
				}
				if model.TimeUntilReset > 0 {
					out.ResetIn = formatDuration(model.TimeUntilReset)
					out.Message += fmt.Sprintf(", resets in %s", out.ResetIn)
				}

				// Add intelligence if available
				if m.tracker != nil {
					summaries, _ := m.tracker.AllUsageSummaries(snap, snap.AccountID)
					for _, s := range summaries {
						if s.ModelID == model.ModelID && s.HasIntelligence {
							out.Rate = fmt.Sprintf("%.1f%%/hr", s.CurrentRate*100)
							out.Message += fmt.Sprintf(", consuming at %s", out.Rate)
						}
					}
				}
				return nil, out, nil
			}
		}
	}

	return nil, ModelAvailOutput{
		Found:   false,
		Message: fmt.Sprintf("No model matching '%s' found. Try: Claude Sonnet, Gemini Pro, GPT, etc.", input.Model),
	}, nil
}

func (m *MCPServer) handleUsageIntelligence(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, IntelligenceOutput, error) {
	snapshots, err := m.store.LatestPerAccount()
	if err != nil {
		return nil, IntelligenceOutput{}, fmt.Errorf("failed to load snapshots: %w", err)
	}

	out := IntelligenceOutput{Message: "Usage intelligence for all tracked models."}

	for _, snap := range snapshots {
		if m.tracker == nil {
			break
		}
		summaries, err := m.tracker.AllUsageSummaries(snap, snap.AccountID)
		if err != nil {
			m.logger.Warn("usage summary error", "error", err)
			continue
		}
		for _, s := range summaries {
			mi := ModelIntel{
				ModelID:          s.ModelID,
				Label:            s.Label,
				Group:            s.Group,
				RemainingPercent: int(math.Round((1 - s.UsagePercent/100) * 100)),
				IsExhausted:      s.IsExhausted,
				HasIntelligence:  s.HasIntelligence,
				CompletedCycles:  s.CompletedCycles,
			}
			if s.TimeUntilReset != "" {
				mi.ResetIn = s.TimeUntilReset
			}
			if s.CycleAge != "" {
				mi.CycleAge = s.CycleAge
			}
			if s.HasIntelligence {
				mi.CurrentRate = fmt.Sprintf("%.1f%%/hr", s.CurrentRate*100)
				if s.ProjectedUsage > 0 {
					mi.ProjectedUsage = fmt.Sprintf("%.0f%%", s.ProjectedUsage*100)
				}
				if s.ProjectedExhaustion != nil {
					mi.ProjectedExhaustion = s.ProjectedExhaustion.Format(time.RFC3339)
				}
			}
			out.Models = append(out.Models, mi)
		}
		// N2: Search ALL accounts, not just first
	}

	if len(out.Models) == 0 {
		out.Message = "No usage data available. Ensure auto-capture is enabled and has been running for at least a few minutes."
	}

	return nil, out, nil
}

func (m *MCPServer) handleBudgetForecast(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, BudgetOutput, error) {
	forecast := tracker.ComputeBudgetForecast(m.store)
	if forecast == nil {
		return nil, BudgetOutput{
			HasBudget: false,
			Message:   "No monthly budget is configured. Set one in the Niyantra dashboard Settings tab.",
		}, nil
	}

	out := BudgetOutput{
		HasBudget:      true,
		MonthlyBudget:  forecast.MonthlyBudget,
		CurrentSpend:   forecast.CurrentSpend,
		ProjectedSpend: forecast.ProjectedMonthlySpend,
		BurnRate:       forecast.BurnRatePerDay,
		OnTrack:        forecast.OnTrack,
	}

	if forecast.OnTrack {
		out.Message = fmt.Sprintf("On track: spending $%.2f/day, projected $%.2f of $%.0f budget.",
			forecast.BurnRatePerDay, forecast.ProjectedMonthlySpend, forecast.MonthlyBudget)
	} else {
		out.Message = fmt.Sprintf("Over budget: spending $%.2f/day, projected $%.2f exceeds $%.0f budget.",
			forecast.BurnRatePerDay, forecast.ProjectedMonthlySpend, forecast.MonthlyBudget)
		if forecast.DaysUntilBudgetExhausted != nil {
			d := *forecast.DaysUntilBudgetExhausted
			out.DaysUntilExhaust = &d
			out.Message += fmt.Sprintf(" Budget exhausts by day %d of month.", d)
		}
	}

	return nil, out, nil
}

func (m *MCPServer) handleBestModel(_ context.Context, _ *mcp.CallToolRequest, input GroupInput) (*mcp.CallToolResult, BestModelOutput, error) {
	group := strings.ToLower(input.Group)
	if group == "" {
		return nil, BestModelOutput{
			Found:  false,
			Reason: "Please specify a group: 'claude_gpt', 'gemini_pro', or 'gemini_flash'.",
		}, nil
	}

	snapshots, err := m.store.LatestPerAccount()
	if err != nil {
		return nil, BestModelOutput{}, fmt.Errorf("failed to load snapshots: %w", err)
	}

	type candidate struct {
		label     string
		remaining float64
		rate      float64
		resetIn   string
		hasRate   bool
	}

	var candidates []candidate

	for _, snap := range snapshots {
		for _, model := range snap.Models {
			modelGroup := client.GroupForModel(model.ModelID, model.Label)
			if strings.ToLower(modelGroup) != group {
				continue
			}
			c := candidate{
				label:     model.Label,
				remaining: model.RemainingFraction,
			}
			if model.TimeUntilReset > 0 {
				c.resetIn = formatDuration(model.TimeUntilReset)
			}

			// Get rate if available
			if m.tracker != nil {
				summaries, _ := m.tracker.AllUsageSummaries(snap, snap.AccountID)
				for _, s := range summaries {
					if s.ModelID == model.ModelID && s.HasIntelligence {
						c.rate = s.CurrentRate
						c.hasRate = true
					}
				}
			}
			candidates = append(candidates, c)
		}
		// N2: Search ALL accounts, not just first
	}

	if len(candidates) == 0 {
		return nil, BestModelOutput{
			Found:  false,
			Reason: fmt.Sprintf("No models found in group '%s'. Available groups: claude_gpt, gemini_pro, gemini_flash.", input.Group),
		}, nil
	}

	// Rank: highest remaining first, lowest rate as tiebreaker
	best := 0
	for i := 1; i < len(candidates); i++ {
		c := candidates[i]
		b := candidates[best]
		if c.remaining > b.remaining {
			best = i
		} else if c.remaining == b.remaining && c.hasRate && b.hasRate && c.rate < b.rate {
			best = i
		}
	}

	out := BestModelOutput{
		Found:       true,
		Recommended: candidates[best].label,
	}

	pct := int(math.Round(candidates[best].remaining * 100))
	if candidates[best].hasRate {
		out.Reason = fmt.Sprintf("%d%% remaining, consuming at %.1f%%/hr", pct, candidates[best].rate*100)
	} else {
		out.Reason = fmt.Sprintf("%d%% remaining", pct)
	}

	for i, c := range candidates {
		if i == best {
			continue
		}
		alt := ModelAlternative{
			Label:     c.label,
			Remaining: int(math.Round(c.remaining * 100)),
			ResetIn:   c.resetIn,
		}
		if c.hasRate {
			alt.Rate = fmt.Sprintf("%.1f%%/hr", c.rate*100)
		}
		out.Alternatives = append(out.Alternatives, alt)
	}

	return nil, out, nil
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

// ── Phase 10 Tool Handlers ───────────────────────────────────────

func (m *MCPServer) handleAnalyzeSpending(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, SpendingOutput, error) {
	overview, err := m.store.SubscriptionOverview()
	if err != nil {
		return nil, SpendingOutput{Message: "Failed to compute spending overview"}, nil
	}

	currency := m.store.GetConfig("currency")
	if currency == "" {
		currency = "USD"
	}

	out := SpendingOutput{
		TotalMonthly:      math.Round(overview.TotalMonthlySpend*100) / 100,
		TotalAnnual:       math.Round(overview.TotalAnnualSpend*100) / 100,
		Currency:          currency,
		SubscriptionCount: m.store.SubscriptionCount(),
	}

	// Category breakdown
	for name, cat := range overview.ByCategory {
		out.Categories = append(out.Categories, CategorySpend{
			Name:    name,
			Monthly: math.Round(cat.MonthlySpend*100) / 100,
			Count:   cat.Count,
		})
	}

	// Generate insights
	insights, err := m.store.GenerateInsights()
	if err == nil && len(insights) > 0 {
		out.Insights = insights
	}

	// Budget status
	budget := m.store.GetConfigFloat("budget_monthly", 0)
	if budget > 0 {
		pct := overview.TotalMonthlySpend / budget * 100
		out.BudgetStatus = &BudgetStatus{
			MonthlyBudget: budget,
			CurrentSpend:  math.Round(overview.TotalMonthlySpend*100) / 100,
			PercentUsed:   math.Round(pct*10) / 10,
			OnTrack:       overview.TotalMonthlySpend <= budget,
		}
	}

	// Summary message
	insightCount := len(out.Insights)
	if insightCount > 0 {
		out.Message = fmt.Sprintf("Tracking %d subscriptions at %s %.2f/month. %d insight(s) found.",
			out.SubscriptionCount, currency, out.TotalMonthly, insightCount)
	} else {
		out.Message = fmt.Sprintf("Tracking %d subscriptions at %s %.2f/month. No issues detected.",
			out.SubscriptionCount, currency, out.TotalMonthly)
	}

	return nil, out, nil
}

func (m *MCPServer) handleSwitchRecommendation(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, SwitchOutput, error) {
	snapshots, err := m.store.LatestPerAccount()
	if err != nil || len(snapshots) == 0 {
		return nil, SwitchOutput{
			Action:  "stay",
			Reason:  "No accounts tracked yet. Capture a snapshot first.",
			Message: "No data available for recommendation.",
		}, nil
	}

	// Build summaries by account for burn rate intelligence
	summariesByAccount := make(map[int64][]*tracker.UsageSummary)
	if m.tracker != nil {
		for _, snap := range snapshots {
			summaries, err := m.tracker.AllUsageSummaries(snap, snap.AccountID)
			if err == nil && len(summaries) > 0 {
				summariesByAccount[snap.AccountID] = summaries
			}
		}
	}

	rec := advisor.Recommend(snapshots, summariesByAccount)

	out := SwitchOutput{
		Action:       rec.Action,
		BestAccount:  rec.BestAccount,
		Alternatives: rec.Alternatives,
		Reason:       rec.Reason,
	}

	switch rec.Action {
	case "switch":
		out.Message = fmt.Sprintf("⚡ Recommendation: SWITCH to %s for better quota availability.", rec.BestAccount.Email)
	case "wait":
		out.Message = "⏳ Recommendation: WAIT — quota resets are imminent."
	default:
		out.Message = fmt.Sprintf("✅ Recommendation: STAY on current account (%s).", rec.BestAccount.Email)
	}

	return nil, out, nil
}

// ── Phase 11 Tool Handlers ───────────────────────────────────────

func (m *MCPServer) handleCodexStatus(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, CodexStatusOutput, error) {
	out := CodexStatusOutput{
		CaptureEnabled: m.store.GetConfigBool("codex_capture"),
	}

	creds, err := codex.DetectCredentials(m.logger)
	if err == nil && creds != nil {
		out.Installed = true
		out.AccountID = creds.AccountID
		if !creds.ExpiresAt.IsZero() {
			out.TokenExpired = creds.IsExpired()
			out.TokenExpiresIn = creds.ExpiresIn.Round(time.Minute).String()
		}
	}

	snap, _ := m.store.LatestCodexSnapshot()
	if snap != nil {
		out.Snapshot = snap
	}

	if !out.Installed {
		out.Message = "Codex CLI not detected. Install Codex and run 'codex auth' to enable quota tracking."
	} else if snap == nil {
		out.Message = fmt.Sprintf("Codex installed (account %s). No snapshots yet — capture one via the dashboard or enable auto-capture.", out.AccountID)
	} else {
		out.Message = fmt.Sprintf("Codex active (account %s). 5h: %.1f%% used, plan: %s.",
			out.AccountID, snap.FiveHourPct, snap.PlanType)
	}

	return nil, out, nil
}

// ForecastGroupOut is a single group's TTX forecast in MCP output.
type ForecastGroupOut struct {
	Group         string `json:"group"`
	BurnRate      string `json:"burnRate"`
	TTX           string `json:"ttx"`
	Remaining     string `json:"remaining"`
	Severity      string `json:"severity"`
	Confidence    string `json:"confidence"`
	EstimatedCost string `json:"estimatedCost,omitempty"`
	CostPerHour   string `json:"costPerHour,omitempty"`
}

// ForecastAccountOut is a single account's forecast in MCP output.
type ForecastAccountOut struct {
	Email    string             `json:"email"`
	PlanName string             `json:"planName"`
	Groups   []ForecastGroupOut `json:"groups"`
}

// ForecastOutput is the output of quota_forecast.
type ForecastOutput struct {
	Antigravity []ForecastAccountOut `json:"antigravity,omitempty"`
	Message     string               `json:"message"`
}

// handleQuotaForecast returns time-to-exhaustion forecasts for all providers.
func (m *MCPServer) handleQuotaForecast(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, ForecastOutput, error) {
	out := ForecastOutput{}

	// Antigravity accounts
	snapshots, err := m.store.LatestPerAccount()
	if err != nil {
		return nil, ForecastOutput{}, fmt.Errorf("database error: %w", err)
	}

	groups := make([]forecast.GroupDefinition, len(client.GroupOrder))
	for i, key := range client.GroupOrder {
		groups[i] = forecast.GroupDefinition{
			GroupKey:     key,
			DisplayName: client.GroupDisplayNames[key],
		}
	}

	assigner := func(modelID string) string {
		return client.GroupForModel(modelID, "")
	}

	for _, snap := range snapshots {
		if snap == nil || snap.AccountID == 0 {
			continue
		}

		recent, err := m.store.RecentModelSnapshots(snap.AccountID, forecast.DefaultWindow)
		if err != nil || len(recent) < 2 {
			continue
		}

		points := make([]forecast.SnapshotPoint, 0, len(recent))
		for _, r := range recent {
			models := forecast.ParseModelsJSON(r.ModelsJSON)
			if models != nil {
				points = append(points, forecast.SnapshotPoint{
					CapturedAt: r.CapturedAt,
					Models:     models,
				})
			}
		}

		if len(points) < 2 {
			continue
		}

		rates := forecast.ComputeRates(points)

		remaining := make(map[string]float64)
		resetTimes := make(map[string]*time.Time)
		for _, mod := range snap.Models {
			remaining[mod.ModelID] = mod.RemainingFraction
			resetTimes[mod.ModelID] = mod.ResetTime
		}

		gf := forecast.ComputeGroupForecasts(rates, remaining, resetTimes, assigner, groups)
		if len(gf) == 0 {
			continue
		}

		af := ForecastAccountOut{
			Email:    snap.Email,
			PlanName: snap.PlanName,
		}
		for _, g := range gf {
			fgo := ForecastGroupOut{
				Group:      g.DisplayName,
				BurnRate:   fmt.Sprintf("%.1f%%/hr", g.BurnRate*100),
				TTX:        g.TTXLabel,
				Remaining:  fmt.Sprintf("%.0f%%", g.Remaining*100),
				Severity:   g.Severity,
				Confidence: g.Confidence,
			}
			// F8: Add cost estimate if we have burn rate data
			if g.BurnRate > 0 {
				costEst := m.computeGroupCost(g.GroupKey, g.BurnRate, g.Remaining)
				if costEst != nil {
					fgo.EstimatedCost = costEst.CostLabel
					fgo.CostPerHour = costEst.HourlyLabel
				}
			}
			af.Groups = append(af.Groups, fgo)
		}
		out.Antigravity = append(out.Antigravity, af)
	}

	if len(out.Antigravity) > 0 {
		out.Message = fmt.Sprintf("Forecast computed for %d Antigravity account(s) using sliding-window analysis (last 60 min).", len(out.Antigravity))
	} else {
		out.Message = "No forecast data available. Need ≥2 snapshots within the last 60 minutes (≥10 min apart) to compute burn rates."
	}

	return nil, out, nil
}

// ── F8: Cost Estimation Helpers ──────────────────────────────────

// computeMCPCosts estimates dollar costs for all accounts using snapshot data.
func (m *MCPServer) computeMCPCosts(snapshots []*client.Snapshot) map[int64]costtrack.AccountCostEstimate {
	if len(snapshots) == 0 {
		return nil
	}

	pricing, err := m.store.GetModelPricing()
	if err != nil {
		return nil
	}

	ceilings, err := costtrack.ParseCeilings(m.store.GetQuotaCeilings())
	if err != nil {
		ceilings = costtrack.DefaultQuotaCeilings()
	}

	ctPricing := make([]costtrack.ModelPricing, len(pricing))
	for i, p := range pricing {
		ctPricing[i] = costtrack.ModelPricing{
			ModelID:     p.ModelID,
			DisplayName: p.DisplayName,
			Provider:    p.Provider,
			InputPer1M:  p.InputPer1M,
			OutputPer1M: p.OutputPer1M,
			CachePer1M:  p.CachePer1M,
		}
	}

	assigner := func(modelID string) string {
		return client.GroupForModel(modelID, "")
	}

	result := make(map[int64]costtrack.AccountCostEstimate)
	for _, snap := range snapshots {
		if snap == nil || snap.AccountID == 0 {
			continue
		}

		// Build rates from snapshot remaining fractions
		groupRemaining := map[string]struct {
			sum   float64
			count int
		}{}
		for _, mod := range snap.Models {
			gk := client.GroupForModel(mod.ModelID, mod.Label)
			acc := groupRemaining[gk]
			acc.sum += mod.RemainingFraction
			acc.count++
			groupRemaining[gk] = acc
		}

		var rates []costtrack.GroupRate
		for _, key := range client.GroupOrder {
			if acc, ok := groupRemaining[key]; ok && acc.count > 0 {
				rates = append(rates, costtrack.GroupRate{
					GroupKey:   key,
					Remaining: acc.sum / float64(acc.count),
					HasData:   true,
				})
			}
		}

		if len(rates) == 0 {
			continue
		}

		est := costtrack.EstimateAccountCost(
			snap.AccountID, snap.Email,
			rates, ceilings, ctPricing, assigner,
		)
		result[snap.AccountID] = est
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// computeGroupCost estimates cost for a single group using its burn rate.
func (m *MCPServer) computeGroupCost(groupKey string, burnRate, remaining float64) *costtrack.GroupCostEstimate {
	pricing, err := m.store.GetModelPricing()
	if err != nil {
		return nil
	}

	ceilings, err := costtrack.ParseCeilings(m.store.GetQuotaCeilings())
	if err != nil {
		ceilings = costtrack.DefaultQuotaCeilings()
	}

	ceiling, ok := ceilings[groupKey]
	if !ok {
		return nil
	}

	ctPricing := make([]costtrack.ModelPricing, len(pricing))
	for i, p := range pricing {
		ctPricing[i] = costtrack.ModelPricing{
			ModelID:     p.ModelID,
			DisplayName: p.DisplayName,
			Provider:    p.Provider,
			InputPer1M:  p.InputPer1M,
			OutputPer1M: p.OutputPer1M,
			CachePer1M:  p.CachePer1M,
		}
	}

	assigner := func(modelID string) string {
		return client.GroupForModel(modelID, "")
	}

	est := costtrack.EstimateGroupCost(
		costtrack.GroupRate{GroupKey: groupKey, BurnRate: burnRate, Remaining: remaining, HasData: true},
		ceiling, ctPricing, assigner,
	)
	return &est
}

// ── Phase 15: Token Usage Analytics (F13) ────────────────────────

// TokenUsageInput is the input for token_usage_stats.
type TokenUsageInput struct {
	Days     int    `json:"days" jsonschema:"number of days to analyze (default 30, max 365)"`
	Provider string `json:"provider" jsonschema:"provider filter: 'all' (default), 'claude', 'antigravity', 'codex', 'cursor', 'gemini'"`
}

// TokenUsageOutput is the output of token_usage_stats.
type TokenUsageOutput struct {
	TotalTokens   int64                      `json:"totalTokens"`
	EstimatedCost float64                    `json:"estimatedCostUSD"`
	InputTokens   int64                      `json:"inputTokens"`
	OutputTokens  int64                      `json:"outputTokens"`
	CacheTokens   int64                      `json:"cacheTokens"`
	Sessions      int                        `json:"sessions"`
	DaysActive    int                        `json:"daysActive"`
	AvgPerDay     int64                      `json:"avgTokensPerDay"`
	CacheHitRate  float64                    `json:"cacheHitRate"`
	TopModel      string                     `json:"topModel"`
	PeakDay       string                     `json:"peakDay"`
	Models        []tokenusage.ModelBreakdown `json:"topModels"`
	Period        tokenusage.Period          `json:"period"`
	Message       string                     `json:"message"`
}

func (m *MCPServer) handleTokenUsageStats(_ context.Context, _ *mcp.CallToolRequest, input TokenUsageInput) (*mcp.CallToolResult, TokenUsageOutput, error) {
	days := input.Days
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	provider := input.Provider
	if provider == "" {
		provider = "all"
	}

	priceFn := func(modelID string) (float64, float64, float64, bool) {
		p := m.store.GetModelPrice(modelID)
		if p == nil {
			return 0, 0, 0, false
		}
		return p.InputPer1M, p.OutputPer1M, p.CachePer1M, true
	}

	var summary *tokenusage.Summary
	var err error

	if provider == "all" || provider == "claude" {
		claudeSummary, cerr := tokenusage.AggregateFromClaude(days, priceFn)
		if cerr != nil {
			m.logger.Warn("MCP token_usage: Claude aggregation failed", "error", cerr)
		}
		if provider == "claude" {
			summary = claudeSummary
		} else {
			storeSummary, _ := tokenusage.AggregateFromStore(m.store, days, "all")
			summary = tokenusage.Merge(claudeSummary, storeSummary)
		}
	} else {
		summary, err = tokenusage.AggregateFromStore(m.store, days, provider)
		if err != nil {
			return nil, TokenUsageOutput{Message: "Failed to aggregate token usage"}, nil
		}
	}

	if summary == nil {
		return nil, TokenUsageOutput{
			Message: "No token usage data available. Ensure Claude Code has been used or other providers have been tracked.",
		}, nil
	}

	// Limit top models to 5 for concise MCP output
	topModels := summary.ByModel
	if len(topModels) > 5 {
		topModels = topModels[:5]
	}

	out := TokenUsageOutput{
		TotalTokens:   summary.Totals.TotalTokens,
		EstimatedCost: summary.Totals.EstCostUSD,
		InputTokens:   summary.Totals.InputTokens,
		OutputTokens:  summary.Totals.OutputTokens,
		CacheTokens:   summary.Totals.CacheTokens,
		Sessions:      summary.Totals.Sessions,
		DaysActive:    summary.KPIs.DaysActive,
		AvgPerDay:     summary.KPIs.AvgTokensPerDay,
		CacheHitRate:  summary.KPIs.CacheHitRate,
		TopModel:      summary.KPIs.TopModel,
		PeakDay:       summary.KPIs.PeakDay,
		Models:        topModels,
		Period:        summary.Period,
	}

	if summary.Totals.TotalTokens > 0 {
		out.Message = fmt.Sprintf("%d days active, %s tokens total, ~$%.2f estimated cost. Top model: %s. Cache hit rate: %.0f%%.",
			summary.KPIs.DaysActive,
			formatTokenCount(summary.Totals.TotalTokens),
			summary.Totals.EstCostUSD,
			summary.KPIs.TopModel,
			summary.KPIs.CacheHitRate*100,
		)
	} else {
		out.Message = "No token usage data found for the specified period."
	}

	return nil, out, nil
}

// formatTokenCount formats large token counts with K/M suffixes.
func formatTokenCount(tokens int64) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	}
	if tokens >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1_000)
	}
	return fmt.Sprintf("%d", tokens)
}

// ── Phase 15: Git Commit Costs (F16) ────────────────────────────

type GitCostInput struct {
	Repo string `json:"repo"`
	Days int    `json:"days"`
}

type GitCostOutput struct {
	Message      string             `json:"message"`
	CommitCount  int                `json:"commitCount"`
	TotalTokens  int64              `json:"totalTokens"`
	TotalCost    float64            `json:"totalCost"`
	AvgPerCommit float64            `json:"avgPerCommit"`
	TopBranch    string             `json:"topBranch"`
	TopCommits   []GitCommitSummary `json:"topCommits,omitempty"`
	Branches     []gitcorr.BranchCost `json:"branches,omitempty"`
}

type GitCommitSummary struct {
	Hash    string  `json:"hash"`
	Message string  `json:"message"`
	CostUSD float64 `json:"costUSD"`
	Tokens  int64   `json:"tokens"`
}

func (m *MCPServer) handleGitCommitCosts(_ context.Context, _ *mcp.CallToolRequest, params GitCostInput) (*mcp.CallToolResult, GitCostOutput, error) {
	repoPath := params.Repo
	if repoPath == "" {
		repoPath = "."
	}
	days := params.Days
	if days <= 0 {
		days = 30
	}

	priceFn := func(modelID string) (float64, float64, float64, bool) {
		p := m.store.GetModelPrice(modelID)
		if p == nil {
			return 0, 0, 0, false
		}
		return p.InputPer1M, p.OutputPer1M, p.CachePer1M, true
	}

	result, err := gitcorr.Analyze(repoPath, days, gitcorr.DefaultWindowMinutes, priceFn)
	if err != nil {
		return nil, GitCostOutput{Message: fmt.Sprintf("Git analysis failed: %v", err)}, nil
	}

	if result == nil || len(result.Commits) == 0 {
		return nil, GitCostOutput{
			Message: "No git commits found in the specified repository and time range.",
		}, nil
	}

	// Top 5 costliest commits
	var topCommits []GitCommitSummary
	for i, c := range result.Commits {
		if i >= 5 {
			break
		}
		if c.CostUSD > 0 {
			topCommits = append(topCommits, GitCommitSummary{
				Hash:    c.ShortHash,
				Message: c.Message,
				CostUSD: c.CostUSD,
				Tokens:  c.TotalTokens,
			})
		}
	}

	// Top 5 branches
	branches := result.Branches
	if len(branches) > 5 {
		branches = branches[:5]
	}

	out := GitCostOutput{
		CommitCount:  result.Totals.CommitCount,
		TotalTokens:  result.Totals.TotalTokens,
		TotalCost:    result.Totals.CostUSD,
		AvgPerCommit: result.Totals.AvgPerCommit,
		TopBranch:    result.Totals.TopBranch,
		TopCommits:   topCommits,
		Branches:     branches,
	}

	if result.Totals.TotalTokens > 0 {
		out.Message = fmt.Sprintf("%d commits analyzed, %s tokens consumed, ~$%.2f total AI cost. Avg $%.2f/commit. Top branch: %s.",
			result.Totals.CommitCount,
			formatTokenCount(result.Totals.TotalTokens),
			result.Totals.CostUSD,
			result.Totals.AvgPerCommit,
			result.Totals.TopBranch,
		)
	} else {
		out.Message = fmt.Sprintf("%d commits found but no overlapping Claude Code session data. Ensure Claude Code has been used for coding in this repository.",
			result.Totals.CommitCount)
	}

	return nil, out, nil
}
