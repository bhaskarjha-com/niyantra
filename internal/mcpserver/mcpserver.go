// Package mcpserver implements a Model Context Protocol (MCP) server
// that exposes Niyantra's quota intelligence to AI coding agents.
//
// The server communicates over stdio (JSON-RPC 2.0) and provides
// 5 tools for querying quota status, model availability, usage
// intelligence, budget forecasts, and model recommendations.
package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
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
func New(s *store.Store, t *tracker.Tracker, logger *slog.Logger) *MCPServer {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "niyantra",
		Version: "1.0.0",
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
	Accounts     []AccountSummary `json:"accounts"`
	AccountCount int              `json:"accountCount"`
	SnapshotCount int             `json:"snapshotCount"`
}

// AccountSummary is a single account in quota_status output.
type AccountSummary struct {
	Email    string         `json:"email"`
	Plan     string         `json:"plan"`
	IsReady  bool           `json:"isReady"`
	Staleness string        `json:"staleness"`
	Groups   []GroupSummary `json:"groups"`
}

// GroupSummary is a quota group within an account.
type GroupSummary struct {
	Name        string `json:"name"`
	GroupKey    string  `json:"groupKey"`
	Remaining  int     `json:"remainingPercent"`
	IsExhausted bool   `json:"isExhausted"`
	ResetIn    string  `json:"resetIn,omitempty"`
}

// ModelAvailOutput is the output of model_availability.
type ModelAvailOutput struct {
	Found     bool    `json:"found"`
	ModelID   string  `json:"modelId,omitempty"`
	Label     string  `json:"label,omitempty"`
	Group     string  `json:"group,omitempty"`
	Available bool    `json:"available"`
	Remaining int     `json:"remainingPercent"`
	ResetIn   string  `json:"resetIn,omitempty"`
	Rate      string  `json:"rate,omitempty"`
	Message   string  `json:"message"`
}

// IntelligenceOutput wraps usage intelligence data.
type IntelligenceOutput struct {
	Models  []ModelIntel `json:"models"`
	Message string       `json:"message"`
}

// ModelIntel is per-model intelligence data.
type ModelIntel struct {
	ModelID             string  `json:"modelId"`
	Label               string  `json:"label"`
	Group               string  `json:"group"`
	RemainingPercent    int     `json:"remainingPercent"`
	IsExhausted         bool    `json:"isExhausted"`
	ResetIn             string  `json:"resetIn,omitempty"`
	CurrentRate         string  `json:"currentRate,omitempty"`
	ProjectedUsage      string  `json:"projectedUsage,omitempty"`
	ProjectedExhaustion string  `json:"projectedExhaustion,omitempty"`
	HasIntelligence     bool    `json:"hasIntelligence"`
	CompletedCycles     int     `json:"completedCycles"`
	CycleAge            string  `json:"cycleAge,omitempty"`
}

// BudgetOutput is the output of budget_forecast.
type BudgetOutput struct {
	HasBudget       bool    `json:"hasBudget"`
	MonthlyBudget   float64 `json:"monthlyBudget,omitempty"`
	CurrentSpend    float64 `json:"currentSpend,omitempty"`
	ProjectedSpend  float64 `json:"projectedMonthlySpend,omitempty"`
	BurnRate        float64 `json:"burnRate,omitempty"`
	OnTrack         bool    `json:"onTrack"`
	DaysUntilExhaust *int   `json:"daysUntilBudgetExhausted,omitempty"`
	Message         string  `json:"message"`
}

// BestModelOutput is the output of best_model.
type BestModelOutput struct {
	Found        bool                `json:"found"`
	Recommended  string              `json:"recommended,omitempty"`
	Reason       string              `json:"reason"`
	Alternatives []ModelAlternative  `json:"alternatives,omitempty"`
}

// ModelAlternative is a single model option in best_model output.
type ModelAlternative struct {
	Label     string `json:"label"`
	Remaining int    `json:"remainingPercent"`
	Rate      string `json:"rate,omitempty"`
	ResetIn   string `json:"resetIn,omitempty"`
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
		for _, g := range acc.Groups {
			gs := GroupSummary{
				Name:        g.DisplayName,
				GroupKey:     g.GroupKey,
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
		break // Use first account
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
		break // First account
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
