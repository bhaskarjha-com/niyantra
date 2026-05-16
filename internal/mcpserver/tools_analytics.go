package mcpserver

import (
	"context"
	"fmt"

	"github.com/bhaskarjha-com/niyantra/internal/gitcorr"
	"github.com/bhaskarjha-com/niyantra/internal/tokenusage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── Token Usage Analytics (F13) ──────────────────────────────────

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

// ── Git Commit Costs (F16) ───────────────────────────────────────

// GitCostInput is the input for git_commit_costs.
type GitCostInput struct {
	Repo string `json:"repo"`
	Days int    `json:"days"`
}

// GitCostOutput is the output of git_commit_costs.
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

// GitCommitSummary is a single commit's cost data.
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

// ── Formatting Helpers ───────────────────────────────────────────

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
