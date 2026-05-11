// Package forecast provides quota time-to-exhaustion (TTX) predictions
// using sliding-window rate calculations over recent snapshot history.
//
// Unlike the older tracker/summary.go approach (which used a cycle-lifetime
// average that diluted during idle periods), this package computes rates
// from actual recent snapshot deltas — producing accurate "current" burn
// rates that reflect the user's real-time usage pattern.
//
// Architecture:
//   - Pure computation — no database writes, no side effects
//   - Accepts snapshot data, returns forecasts
//   - Used by /api/status (inline TTX), /api/forecast (detailed), and MCP
package forecast

import (
	"encoding/json"
	"math"
	"strconv"
	"time"
)

// DefaultWindow is the lookback duration for rate calculations.
// 60 minutes provides enough data points (at 5-min poll intervals = ~12 points)
// while being responsive to changes in usage pattern.
const DefaultWindow = 60 * time.Minute

// MinDataPoints is the minimum number of snapshot pairs needed to compute a rate.
// With fewer points, we can't distinguish signal from noise.
const MinDataPoints = 2

// MinTimeSpan is the minimum time span between first and last snapshot.
// Prevents noisy extrapolation from near-simultaneous snapshots.
const MinTimeSpan = 10 * time.Minute

// SnapshotPoint represents a single point in time with per-model remaining fractions.
type SnapshotPoint struct {
	CapturedAt time.Time
	Models     map[string]float64 // modelID → remainingFraction (0.0–1.0)
}

// GroupForecast is the TTX prediction for a single quota group.
type GroupForecast struct {
	GroupKey     string  `json:"groupKey"`
	DisplayName string  `json:"displayName"`
	BurnRate    float64 `json:"burnRate"`    // fraction consumed per hour (0.0–1.0 scale)
	TTXHours    float64 `json:"ttxHours"`    // hours until exhaustion (-1 = no data, 0 = exhausted)
	TTXLabel    string  `json:"ttxLabel"`    // human-readable "~2.3h left", "~45m left"
	Remaining   float64 `json:"remaining"`   // current remaining fraction (0.0–1.0)
	Confidence  string  `json:"confidence"`  // "high" (≥6 points), "medium" (3–5), "low" (2)
	WillExhaust bool    `json:"willExhaust"` // true if projected to exhaust before reset
	Severity    string  `json:"severity"`    // "safe", "caution", "warning", "critical"
}

// AccountForecast is the TTX prediction for a single account.
type AccountForecast struct {
	AccountID int64           `json:"accountId"`
	Email     string          `json:"email"`
	Groups    []GroupForecast `json:"groups"`
}

// ModelRate holds the computed rate for a single model.
type ModelRate struct {
	ModelID  string
	Rate     float64 // fraction consumed per hour
	Points   int     // number of data points used
	TimeSpan time.Duration
}

// GroupDefinition maps a group key to its display name.
type GroupDefinition struct {
	GroupKey     string
	DisplayName string
}

// GroupAssigner is a function that returns the group key for a model ID.
type GroupAssigner func(modelID string) string

// ComputeRates calculates per-model burn rates from recent snapshot history
// using a sliding-window approach with recency weighting.
//
// Algorithm:
//  1. For each consecutive snapshot pair (t1, t2), compute Δfraction/Δhours
//  2. Only count DECREASES (usage) — increases are resets/corrections
//  3. Weight recent deltas higher: weight = 1 + (index / total) to give
//     the latest pair ~2x the weight of the earliest pair
//  4. Return weighted average rate per model
func ComputeRates(points []SnapshotPoint) map[string]*ModelRate {
	if len(points) < MinDataPoints {
		return nil
	}

	// Ensure chronological order
	// (caller should provide ASC order, but be defensive)

	// Check minimum time span
	totalSpan := points[len(points)-1].CapturedAt.Sub(points[0].CapturedAt)
	if totalSpan < MinTimeSpan {
		return nil
	}

	// Collect all model IDs across all points
	modelIDs := map[string]bool{}
	for _, p := range points {
		for id := range p.Models {
			modelIDs[id] = true
		}
	}

	rates := map[string]*ModelRate{}
	for modelID := range modelIDs {
		rate := computeModelRate(modelID, points)
		if rate != nil {
			rates[modelID] = rate
		}
	}

	return rates
}

// computeModelRate calculates the burn rate for a single model from snapshot pairs.
func computeModelRate(modelID string, points []SnapshotPoint) *ModelRate {
	type delta struct {
		rate  float64 // fraction per hour
		index int     // position in sequence (for weighting)
	}

	var deltas []delta
	pairCount := 0

	for i := 1; i < len(points); i++ {
		prev := points[i-1]
		curr := points[i]

		prevFrac, prevOK := prev.Models[modelID]
		currFrac, currOK := curr.Models[modelID]
		if !prevOK || !currOK {
			continue
		}

		dt := curr.CapturedAt.Sub(prev.CapturedAt)
		if dt <= 0 {
			continue
		}

		pairCount++
		consumed := prevFrac - currFrac // positive = usage occurred

		if consumed <= 0 {
			// Fraction increased or stayed same — no consumption in this interval.
			// Could be a reset, correction, or idle period.
			// Count as zero-rate data point to properly account for idle time.
			deltas = append(deltas, delta{rate: 0, index: i - 1})
			continue
		}

		hours := dt.Hours()
		if hours <= 0 {
			continue
		}

		deltas = append(deltas, delta{
			rate:  consumed / hours,
			index: i - 1,
		})
	}

	if len(deltas) == 0 {
		return nil
	}

	// Weighted average: more recent deltas get higher weight.
	// weight = 1 + (position / total), so latest pair gets ~2x weight of earliest.
	totalWeight := 0.0
	weightedSum := 0.0

	for _, d := range deltas {
		weight := 1.0 + float64(d.index)/float64(pairCount)
		weightedSum += d.rate * weight
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return nil
	}

	avgRate := weightedSum / totalWeight
	timeSpan := points[len(points)-1].CapturedAt.Sub(points[0].CapturedAt)

	return &ModelRate{
		ModelID:  modelID,
		Rate:     avgRate,
		Points:   len(deltas) + 1, // deltas + 1 = snapshot count
		TimeSpan: timeSpan,
	}
}

// ComputeGroupForecasts aggregates per-model rates into per-group TTX predictions.
func ComputeGroupForecasts(
	rates map[string]*ModelRate,
	currentRemaining map[string]float64, // modelID → current remaining fraction
	resetTimes map[string]*time.Time, // modelID → reset time (nil if unknown)
	groupAssigner GroupAssigner,
	groups []GroupDefinition,
) []GroupForecast {
	if len(rates) == 0 && len(currentRemaining) == 0 {
		return nil
	}

	// Aggregate by group
	type groupAcc struct {
		totalRate      float64
		totalRemaining float64
		rateCount      int
		remainCount    int
		maxPoints      int
		earliestReset  *time.Time
	}

	byGroup := map[string]*groupAcc{}
	for _, g := range groups {
		byGroup[g.GroupKey] = &groupAcc{}
	}

	// Accumulate rates
	for modelID, rate := range rates {
		gk := groupAssigner(modelID)
		acc, ok := byGroup[gk]
		if !ok {
			acc = &groupAcc{}
			byGroup[gk] = acc
		}
		acc.totalRate += rate.Rate
		acc.rateCount++
		if rate.Points > acc.maxPoints {
			acc.maxPoints = rate.Points
		}
	}

	// Accumulate remaining fractions + reset times
	for modelID, frac := range currentRemaining {
		gk := groupAssigner(modelID)
		acc, ok := byGroup[gk]
		if !ok {
			acc = &groupAcc{}
			byGroup[gk] = acc
		}
		acc.totalRemaining += frac
		acc.remainCount++

		if rt, ok := resetTimes[modelID]; ok && rt != nil {
			if acc.earliestReset == nil || rt.Before(*acc.earliestReset) {
				t := *rt
				acc.earliestReset = &t
			}
		}
	}

	var forecasts []GroupForecast
	for _, gd := range groups {
		acc := byGroup[gd.GroupKey]
		if acc == nil {
			continue
		}

		f := GroupForecast{
			GroupKey:     gd.GroupKey,
			DisplayName:  gd.DisplayName,
			TTXHours:     -1, // default: no data
			Confidence:   "none",
			Severity:     "safe",
		}

		// Average remaining across models in group
		if acc.remainCount > 0 {
			f.Remaining = acc.totalRemaining / float64(acc.remainCount)
		}

		// Average rate across models in group
		if acc.rateCount > 0 {
			f.BurnRate = acc.totalRate / float64(acc.rateCount)
		}

		// Confidence level based on data points
		switch {
		case acc.maxPoints >= 6:
			f.Confidence = "high"
		case acc.maxPoints >= 3:
			f.Confidence = "medium"
		case acc.maxPoints >= 2:
			f.Confidence = "low"
		default:
			f.Confidence = "none"
		}

		// Compute TTX
		if f.BurnRate > 0 && f.Remaining > 0 {
			f.TTXHours = f.Remaining / f.BurnRate
			f.TTXLabel = formatTTX(f.TTXHours)

			// Check if will exhaust before reset
			if acc.earliestReset != nil {
				hoursToReset := time.Until(*acc.earliestReset).Hours()
				if hoursToReset > 0 {
					f.WillExhaust = f.TTXHours < hoursToReset
				}
			}
		} else if f.Remaining <= 0 {
			f.TTXHours = 0
			f.TTXLabel = "exhausted"
		} else if f.BurnRate <= 0 && f.Confidence != "none" {
			// Rate is zero but we have data — user is idle
			f.TTXLabel = "idle"
			f.TTXHours = -1
		}

		// Severity
		if f.TTXHours == 0 {
			f.Severity = "critical"
		} else if f.TTXHours > 0 {
			switch {
			case f.TTXHours < 0.5:
				f.Severity = "critical"
			case f.TTXHours < 1.0:
				f.Severity = "warning"
			case f.TTXHours < 3.0:
				f.Severity = "caution"
			default:
				f.Severity = "safe"
			}
		}

		forecasts = append(forecasts, f)
	}

	return forecasts
}

// formatTTX produces a human-readable TTX label like "~2.3h", "~45m", "~8m".
func formatTTX(hours float64) string {
	if hours <= 0 {
		return "exhausted"
	}
	if hours >= 24 {
		days := hours / 24
		if days >= 2 {
			return "~" + formatFloat(days) + "d"
		}
		return "~" + formatFloat(hours) + "h"
	}
	if hours >= 1 {
		return "~" + formatFloat(hours) + "h"
	}
	mins := hours * 60
	if mins < 1 {
		return "<1m"
	}
	return "~" + formatFloat(mins) + "m"
}

// FormatTTXPublic is the exported version of formatTTX for use by other packages.
func FormatTTXPublic(hours float64) string {
	return formatTTX(hours)
}

// formatFloat formats a number to 1 decimal place, stripping trailing ".0".
func formatFloat(v float64) string {
	rounded := math.Round(v*10) / 10
	if rounded == math.Floor(rounded) {
		return strconv.Itoa(int(rounded))
	}
	return strconv.FormatFloat(rounded, 'f', 1, 64)
}

// ParseModelsJSON parses a models_json string into a map of modelID → remainingFraction.
func ParseModelsJSON(modelsJSON string) map[string]float64 {
	if modelsJSON == "" {
		return nil
	}

	type modelEntry struct {
		ModelID           string  `json:"modelId"`
		RemainingFraction float64 `json:"remainingFraction"`
	}

	var models []modelEntry
	if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
		return nil
	}

	result := make(map[string]float64, len(models))
	for _, m := range models {
		if m.ModelID != "" {
			result[m.ModelID] = m.RemainingFraction
		}
	}
	return result
}
