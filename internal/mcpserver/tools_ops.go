package mcpserver

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/advisor"
	"github.com/bhaskarjha-com/niyantra/internal/codex"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tracker"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── Ops Tool Output Types ────────────────────────────────────────

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

// ── Ops Tool Handlers ────────────────────────────────────────────

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
