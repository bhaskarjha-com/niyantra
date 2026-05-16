package mcpserver

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── Quota Tool Input/Output Types ────────────────────────────────

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

// ── Quota Tool Handlers ──────────────────────────────────────────

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
