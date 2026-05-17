package web

import (
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/costtrack"
	"github.com/bhaskarjha-com/niyantra/internal/forecast"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// computeAccountCosts estimates dollar costs for each account using
// burn rates from the forecast engine + model pricing from F5.
// N24: The frontend should clarify the difference between
// "Subscription Forecast" (fixed monthly costs) and "Estimated Token Cost"
// (variable usage × model pricing). Currently these are conflated in the Overview tab.
func (s *Server) computeAccountCosts(
	snapshots []*client.Snapshot,
	forecasts map[int64][]forecast.GroupForecast,
) map[int64]costtrack.AccountCostEstimate {
	if len(snapshots) == 0 {
		return nil
	}

	// Load pricing and ceilings from config
	pricing, err := s.store.GetModelPricing()
	if err != nil {
		s.logger.Warn("cost tracking: pricing load failed", "error", err)
		return nil
	}

	ceilings, err := costtrack.ParseCeilings(s.store.GetQuotaCeilings())
	if err != nil {
		s.logger.Warn("cost tracking: ceilings parse failed", "error", err)
		ceilings = costtrack.DefaultQuotaCeilings()
	}

	// Convert store.ModelPrice to costtrack.ModelPricing
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

		// Build GroupRate from forecast data (if available) or from latest snapshot
		var rates []costtrack.GroupRate

		if forecastGroups, ok := forecasts[snap.AccountID]; ok && len(forecastGroups) > 0 {
			// Use forecast data (has burn rates)
			for _, fg := range forecastGroups {
				rates = append(rates, costtrack.GroupRate{
					GroupKey:  fg.GroupKey,
					BurnRate: fg.BurnRate,
					Remaining: fg.Remaining,
					HasData:   fg.Confidence != "none",
				})
			}
		} else {
			// No forecast data — compute remaining from latest snapshot
			groupRemaining := map[string]struct {
				sum   float64
				count int
			}{}
			for _, m := range snap.Models {
				gk := client.GroupForModel(m.ModelID, m.Label)
				acc := groupRemaining[gk]
				acc.sum += m.RemainingFraction
				acc.count++
				groupRemaining[gk] = acc
			}
			for _, key := range client.GroupOrder {
				if acc, ok := groupRemaining[key]; ok && acc.count > 0 {
					rates = append(rates, costtrack.GroupRate{
						GroupKey:  key,
						Remaining: acc.sum / float64(acc.count),
						HasData:   true,
					})
				}
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

// computeAccountForecasts builds sliding-window TTX forecasts for all accounts.
// Returns a map of accountID → []GroupForecast for inline enrichment in /api/status.
func (s *Server) computeAccountForecasts(snapshots []*client.Snapshot) map[int64][]forecast.GroupForecast {
	if len(snapshots) == 0 {
		return nil
	}

	groups := make([]forecast.GroupDefinition, len(client.GroupOrder))
	for i, key := range client.GroupOrder {
		groups[i] = forecast.GroupDefinition{
			GroupKey:    key,
			DisplayName: client.GroupDisplayNames[key],
		}
	}

	assigner := func(modelID string) string {
		return client.GroupForModel(modelID, "")
	}

	result := make(map[int64][]forecast.GroupForecast)

	for _, snap := range snapshots {
		if snap == nil || snap.AccountID == 0 {
			continue
		}

		// Get recent snapshots for this account (last 60 min)
		recent, err := s.store.RecentModelSnapshots(snap.AccountID, forecast.DefaultWindow)
		if err != nil || len(recent) < 2 {
			continue
		}

		// Convert to forecast.SnapshotPoint
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

		// Compute rates from recent history
		rates := forecast.ComputeRates(points)

		// Build current remaining + reset times from latest snapshot
		// Apply stale-correction: if a model's reset time is in the past,
		// the quota has refilled — infer remaining = 1.0 to match readiness display.
		remaining := make(map[string]float64)
		resetTimes := make(map[string]*time.Time)
		now := time.Now()
		for _, m := range snap.Models {
			frac := m.RemainingFraction
			if m.ResetTime != nil && m.ResetTime.Before(now) && frac <= 0 {
				// Reset time passed and model was exhausted → quota has refilled
				frac = 1.0
			}
			remaining[m.ModelID] = frac
			resetTimes[m.ModelID] = m.ResetTime
		}

		// Compute group-level forecasts
		gf := forecast.ComputeGroupForecasts(rates, remaining, resetTimes, assigner, groups)
		if len(gf) > 0 {
			result[snap.AccountID] = gf
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// computeClaudeForecasts computes TTX for Claude Code from recent snapshots.
func (s *Server) computeClaudeForecasts() map[string]interface{} {
	recent, err := s.store.RecentClaudeSnapshots(forecast.DefaultWindow)
	if err != nil || len(recent) < 2 {
		return nil
	}

	// Build snapshot points for 5-hour and 7-day windows
	type claudeForecast struct {
		Window   string  `json:"window"`
		BurnRate float64 `json:"burnRate"` // pct/hr
		TTXHours float64 `json:"ttxHours"`
		TTXLabel string  `json:"ttxLabel"`
		Severity string  `json:"severity"`
		Used     float64 `json:"used"`
	}

	var forecasts []claudeForecast

	// 5-hour window
	if rate, remaining := computeSimpleRate(recent, func(s store.ClaudeSnapshot) float64 {
		return 100 - s.FiveHourPct // FiveHourPct is % used, we need % remaining
	}); rate > 0 && remaining >= 0 {
		ttx := remaining / rate
		forecasts = append(forecasts, claudeForecast{
			Window:   "5-hour",
			BurnRate: rate,
			TTXHours: ttx,
			TTXLabel: forecast.FormatTTXPublic(ttx),
			Severity: ttxSeverity(ttx),
			Used:     100 - remaining,
		})
	}

	// 7-day window
	if rate, remaining := computeSimpleRate(recent, func(s store.ClaudeSnapshot) float64 {
		if s.SevenDayPct != nil {
			return 100 - *s.SevenDayPct
		}
		return -1 // no data
	}); rate > 0 && remaining >= 0 {
		ttx := remaining / rate
		forecasts = append(forecasts, claudeForecast{
			Window:   "7-day",
			BurnRate: rate,
			TTXHours: ttx,
			TTXLabel: forecast.FormatTTXPublic(ttx),
			Severity: ttxSeverity(ttx),
			Used:     100 - remaining,
		})
	}

	if len(forecasts) == 0 {
		return nil
	}
	return map[string]interface{}{
		"windows": forecasts,
	}
}

// computeCodexForecasts computes TTX for Codex from recent snapshots.
func (s *Server) computeCodexForecasts() map[string]interface{} {
	recent, err := s.store.RecentCodexSnapshots(forecast.DefaultWindow)
	if err != nil || len(recent) < 2 {
		return nil
	}

	type codexForecast struct {
		Window   string  `json:"window"`
		BurnRate float64 `json:"burnRate"`
		TTXHours float64 `json:"ttxHours"`
		TTXLabel string  `json:"ttxLabel"`
		Severity string  `json:"severity"`
		Used     float64 `json:"used"`
	}

	var forecasts []codexForecast

	// 5-hour window
	if rate, remaining := computeCodexRate(recent, func(s *store.CodexSnapshot) float64 {
		return 100 - s.FiveHourPct
	}); rate > 0 && remaining >= 0 {
		ttx := remaining / rate
		forecasts = append(forecasts, codexForecast{
			Window:   "5-hour",
			BurnRate: rate,
			TTXHours: ttx,
			TTXLabel: forecast.FormatTTXPublic(ttx),
			Severity: ttxSeverity(ttx),
			Used:     100 - remaining,
		})
	}

	if len(forecasts) == 0 {
		return nil
	}
	return map[string]interface{}{
		"windows": forecasts,
	}
}

// computeSimpleRate computes a weighted-average rate from Claude snapshots.
// Returns (rate in pct/hr, current remaining pct). Rate is 0 if no decrease detected.
func computeSimpleRate(snaps []store.ClaudeSnapshot, extractor func(store.ClaudeSnapshot) float64) (float64, float64) {
	if len(snaps) < 2 {
		return 0, -1
	}

	totalWeight := 0.0
	weightedRate := 0.0

	for i := 1; i < len(snaps); i++ {
		prev := extractor(snaps[i-1])
		curr := extractor(snaps[i])
		if prev < 0 || curr < 0 {
			continue
		}

		dt := snaps[i].CapturedAt.Sub(snaps[i-1].CapturedAt)
		if dt <= 0 {
			continue
		}

		consumed := prev - curr // positive = usage
		if consumed < 0 {
			consumed = 0 // reset or correction
		}

		rate := consumed / dt.Hours()
		weight := 1.0 + float64(i-1)/float64(len(snaps)-1)
		weightedRate += rate * weight
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return 0, -1
	}

	lastRemaining := extractor(snaps[len(snaps)-1])
	return weightedRate / totalWeight, lastRemaining
}

// computeCodexRate computes a weighted-average rate from Codex snapshots.
func computeCodexRate(snaps []*store.CodexSnapshot, extractor func(*store.CodexSnapshot) float64) (float64, float64) {
	if len(snaps) < 2 {
		return 0, -1
	}

	totalWeight := 0.0
	weightedRate := 0.0

	for i := 1; i < len(snaps); i++ {
		prev := extractor(snaps[i-1])
		curr := extractor(snaps[i])
		if prev < 0 || curr < 0 {
			continue
		}

		dt := snaps[i].CapturedAt.Sub(snaps[i-1].CapturedAt)
		if dt <= 0 {
			continue
		}

		consumed := prev - curr
		if consumed < 0 {
			consumed = 0
		}

		rate := consumed / dt.Hours()
		weight := 1.0 + float64(i-1)/float64(len(snaps)-1)
		weightedRate += rate * weight
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return 0, -1
	}

	lastRemaining := extractor(snaps[len(snaps)-1])
	return weightedRate / totalWeight, lastRemaining
}

// ttxSeverity returns the severity level for a given TTX in hours.
func ttxSeverity(hours float64) string {
	switch {
	case hours <= 0:
		return "critical"
	case hours < 0.5:
		return "critical"
	case hours < 1.0:
		return "warning"
	case hours < 3.0:
		return "caution"
	default:
		return "safe"
	}
}
