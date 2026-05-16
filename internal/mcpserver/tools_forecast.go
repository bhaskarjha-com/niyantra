package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/costtrack"
	"github.com/bhaskarjha-com/niyantra/internal/forecast"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── Forecast Tool Output Types ───────────────────────────────────

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

// ── Forecast Tool Handlers ───────────────────────────────────────

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
