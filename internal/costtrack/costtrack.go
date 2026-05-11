// Package costtrack provides estimated cost calculations from quota fraction
// changes and configurable model pricing.
//
// Unlike tools that parse raw token counts from local logs (cc-statistics,
// ccstats), Niyantra tracks quota fractions (0.0–1.0). This package estimates
// dollar cost by:
//
//  1. Computing Δ remainingFraction between snapshot pairs (from forecast engine)
//  2. Mapping fraction consumed → estimated tokens via configurable quota ceilings
//  3. Applying per-model pricing (from F5 Model Pricing Config)
//
// Architecture:
//   - Pure computation — no database writes, no side effects
//   - Accepts rates + pricing + ceilings, returns cost estimates
//   - Used by /api/status (inline costs), /api/cost (detailed), and MCP
package costtrack

import (
	"encoding/json"
	"fmt"
	"math"
)

// DefaultQuotaCeilings returns the default tokens-per-5h-cycle assumptions.
// These are rough estimates for Antigravity Pro+ plans (May 2026).
// Users can override via the settings UI.
func DefaultQuotaCeilings() map[string]GroupCeiling {
	return map[string]GroupCeiling{
		"claude_gpt": {
			GroupKey:            "claude_gpt",
			DisplayName:        "Claude + GPT",
			TokensPerCycle:     5_000_000,
			CycleDurationHours: 5,
		},
		"gemini_pro": {
			GroupKey:            "gemini_pro",
			DisplayName:        "Gemini Pro",
			TokensPerCycle:     3_000_000,
			CycleDurationHours: 5,
		},
		"gemini_flash": {
			GroupKey:            "gemini_flash",
			DisplayName:        "Gemini Flash",
			TokensPerCycle:     10_000_000,
			CycleDurationHours: 5,
		},
	}
}

// GroupCeiling defines the assumed token capacity for a quota group's cycle.
type GroupCeiling struct {
	GroupKey            string  `json:"groupKey"`
	DisplayName        string  `json:"displayName"`
	TokensPerCycle     float64 `json:"tokensPerCycle"`     // total tokens per reset cycle
	CycleDurationHours float64 `json:"cycleDurationHours"` // cycle length in hours
}

// ModelPricing holds per-model pricing in $/1M tokens.
// Mirrors store.ModelPrice but decoupled from the store package.
type ModelPricing struct {
	ModelID     string  `json:"modelId"`
	DisplayName string  `json:"displayName"`
	Provider    string  `json:"provider"`
	InputPer1M  float64 `json:"inputPer1M"`
	OutputPer1M float64 `json:"outputPer1M"`
	CachePer1M  float64 `json:"cachePer1M"`
}

// BlendedPricePerToken returns a blended $/token rate assuming a typical
// coding-assistant usage split: 40% input tokens, 60% output tokens.
// Cache pricing is excluded since it's a minor optimization.
func (p ModelPricing) BlendedPricePerToken() float64 {
	blendedPer1M := 0.4*p.InputPer1M + 0.6*p.OutputPer1M
	return blendedPer1M / 1_000_000
}

// GroupCostEstimate is the estimated cost for a single quota group.
type GroupCostEstimate struct {
	GroupKey      string  `json:"groupKey"`
	DisplayName   string  `json:"displayName"`
	ConsumedFrac  float64 `json:"consumedFraction"`  // fraction consumed in current window
	EstTokens     float64 `json:"estimatedTokens"`    // estimated tokens consumed
	EstCost       float64 `json:"estimatedCost"`      // $ cost estimate
	CostPerHour   float64 `json:"costPerHour"`         // $/hr at current burn rate
	CostLabel     string  `json:"costLabel"`           // human-readable "$1.23"
	HourlyLabel   string  `json:"hourlyLabel"`         // "$0.41/hr"
	HasData       bool    `json:"hasData"`
}

// AccountCostEstimate is the estimated cost for a single account.
type AccountCostEstimate struct {
	AccountID    int64               `json:"accountId"`
	Email        string              `json:"email"`
	TotalCost    float64             `json:"totalCost"`
	TotalLabel   string              `json:"totalLabel"`
	Groups       []GroupCostEstimate `json:"groups"`
}

// CostSummary is the aggregate estimated cost across all accounts.
type CostSummary struct {
	TotalCostToday float64 `json:"totalCostToday"` // sum across all accounts
	TotalLabel     string  `json:"totalLabel"`      // "$4.56"
}

// GroupRate holds per-group burn rate data from the forecast engine.
type GroupRate struct {
	GroupKey   string
	BurnRate  float64 // fraction consumed per hour
	Remaining float64 // current remaining fraction (0.0–1.0)
	HasData   bool
}

// EstimateGroupCost computes the estimated dollar cost for a single group
// based on its burn rate, quota ceiling, and model pricing.
//
// Algorithm:
//  1. consumed_per_hour = burnRate (fraction/hr)
//  2. tokens_per_hour = consumed_per_hour × tokensPerCycle
//  3. cost_per_hour = tokens_per_hour × blended_price_per_token
//  4. cost_this_cycle = (1 - remaining) × tokensPerCycle × blended_price_per_token
func EstimateGroupCost(
	rate GroupRate,
	ceiling GroupCeiling,
	pricing []ModelPricing,
	groupAssigner func(modelID string) string,
) GroupCostEstimate {
	est := GroupCostEstimate{
		GroupKey:     rate.GroupKey,
		DisplayName: ceiling.DisplayName,
		HasData:     rate.HasData,
	}

	if !rate.HasData || ceiling.TokensPerCycle <= 0 {
		est.CostLabel = "—"
		est.HourlyLabel = "—"
		return est
	}

	// Find blended price for models in this group
	blendedPrice := blendedPriceForGroup(rate.GroupKey, pricing, groupAssigner)
	if blendedPrice <= 0 {
		est.CostLabel = "—"
		est.HourlyLabel = "—"
		return est
	}

	// Consumed fraction = 1 - remaining
	consumedFrac := 1.0 - rate.Remaining
	if consumedFrac < 0 {
		consumedFrac = 0
	}

	// Estimate tokens consumed this cycle
	estTokens := consumedFrac * ceiling.TokensPerCycle

	// Estimate cost
	estCost := estTokens * blendedPrice

	// Hourly rate
	tokensPerHour := rate.BurnRate * ceiling.TokensPerCycle
	costPerHour := tokensPerHour * blendedPrice

	est.ConsumedFrac = consumedFrac
	est.EstTokens = math.Round(estTokens)
	est.EstCost = math.Round(estCost*100) / 100
	est.CostPerHour = math.Round(costPerHour*100) / 100
	est.CostLabel = formatCost(est.EstCost)
	est.HourlyLabel = formatCost(est.CostPerHour) + "/hr"

	return est
}

// EstimateAccountCost computes the total estimated cost for an account
// across all its quota groups.
func EstimateAccountCost(
	accountID int64,
	email string,
	rates []GroupRate,
	ceilings map[string]GroupCeiling,
	pricing []ModelPricing,
	groupAssigner func(modelID string) string,
) AccountCostEstimate {
	est := AccountCostEstimate{
		AccountID: accountID,
		Email:     email,
	}

	for _, rate := range rates {
		ceiling, ok := ceilings[rate.GroupKey]
		if !ok {
			continue
		}
		ge := EstimateGroupCost(rate, ceiling, pricing, groupAssigner)
		est.Groups = append(est.Groups, ge)
		est.TotalCost += ge.EstCost
	}

	est.TotalCost = math.Round(est.TotalCost*100) / 100
	est.TotalLabel = formatCost(est.TotalCost)

	return est
}

// blendedPriceForGroup computes the average blended price across all models
// assigned to a given group key.
func blendedPriceForGroup(groupKey string, pricing []ModelPricing, groupAssigner func(string) string) float64 {
	var total float64
	var count int

	for _, p := range pricing {
		if groupAssigner(p.ModelID) == groupKey {
			bp := p.BlendedPricePerToken()
			if bp > 0 {
				total += bp
				count++
			}
		}
	}

	if count == 0 {
		// Fallback: use average of all pricing as rough estimate
		for _, p := range pricing {
			bp := p.BlendedPricePerToken()
			if bp > 0 {
				total += bp
				count++
			}
		}
		if count == 0 {
			return 0
		}
		// Divide total by number of groups (~3) to avoid over-counting
		return total / float64(count) / 3
	}

	return total / float64(count)
}

// formatCost produces a human-readable cost label like "$1.23", "$0.05", "$12.50".
func formatCost(amount float64) string {
	if amount <= 0 {
		return "$0.00"
	}
	if amount < 0.01 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", amount)
}

// FormatCost is the exported version of formatCost for use by other packages.
func FormatCost(amount float64) string {
	return formatCost(amount)
}

// ParseCeilings parses quota ceilings from a JSON string (config table).
func ParseCeilings(raw string) (map[string]GroupCeiling, error) {
	if raw == "" {
		return DefaultQuotaCeilings(), nil
	}

	var ceilings map[string]GroupCeiling
	if err := json.Unmarshal([]byte(raw), &ceilings); err != nil {
		return nil, fmt.Errorf("costtrack: parsing ceilings: %w", err)
	}
	return ceilings, nil
}

// MarshalCeilings serializes quota ceilings to JSON for storage.
func MarshalCeilings(ceilings map[string]GroupCeiling) (string, error) {
	data, err := json.Marshal(ceilings)
	if err != nil {
		return "", fmt.Errorf("costtrack: marshalling ceilings: %w", err)
	}
	return string(data), nil
}
