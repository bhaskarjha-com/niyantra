package tracker

import (
	"fmt"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// UsageSummary contains computed usage statistics for an Antigravity model.
type UsageSummary struct {
	ModelID           string     `json:"modelId"`
	Label             string     `json:"label"`
	Group             string     `json:"group"`
	RemainingFraction float64    `json:"remainingFraction"`
	UsagePercent      float64    `json:"usagePercent"`
	IsExhausted       bool       `json:"isExhausted"`
	ResetTime         *time.Time `json:"resetTime"`
	TimeUntilReset    string     `json:"timeUntilReset"`

	// Intelligence (requires ≥30 min of data)
	CurrentRate         float64    `json:"currentRate"`         // usage fraction per hour
	ProjectedUsage      float64    `json:"projectedUsage"`      // projected usage at reset (0.0-1.0)
	ProjectedExhaustion *time.Time `json:"projectedExhaustion"` // when quota hits 0 at current rate
	HasIntelligence     bool       `json:"hasIntelligence"`     // true if rate data is available

	// History
	CompletedCycles int     `json:"completedCycles"`
	AvgPerCycle     float64 `json:"avgPerCycle"`
	PeakCycle       float64 `json:"peakCycle"`
	TotalTracked    float64 `json:"totalTracked"`

	// Active cycle info
	CycleAge       string `json:"cycleAge"`
	CycleSnapshots int    `json:"cycleSnapshots"`
}

// BudgetForecast provides budget burn rate projections.
type BudgetForecast struct {
	MonthlyBudget            float64 `json:"monthlyBudget"`
	CurrentSpend             float64 `json:"currentSpend"`
	ProjectedMonthlySpend    float64 `json:"projectedMonthlySpend"`
	BurnRatePerDay           float64 `json:"burnRate"`
	DaysUntilBudgetExhausted *int    `json:"daysUntilBudgetExhausted"`
	OnTrack                  bool    `json:"onTrack"`
	DayOfMonth               int     `json:"dayOfMonth"`
	DaysInMonth              int     `json:"daysInMonth"`
}

// UsageSummaryForModel computes intelligence for a single model.
func (t *Tracker) UsageSummaryForModel(modelID string, accountID int64, model client.ModelQuota) (*UsageSummary, error) {
	summary := &UsageSummary{
		ModelID:           model.ModelID,
		Label:             model.Label,
		Group:             client.GroupForModel(model.ModelID, model.Label),
		RemainingFraction: model.RemainingFraction,
		UsagePercent:      (1.0 - model.RemainingFraction) * 100,
		IsExhausted:       model.IsExhausted,
		ResetTime:         model.ResetTime,
	}

	if model.ResetTime != nil {
		remaining := time.Until(*model.ResetTime)
		if remaining < 0 {
			remaining = 0
		}
		summary.TimeUntilReset = formatDuration(remaining)
	}

	// Get active cycle
	activeCycle, err := t.store.ActiveCycle(modelID, accountID)
	if err != nil {
		return summary, nil // Return basic summary without intelligence
	}

	// Get completed history
	history, err := t.store.CycleHistory(modelID, accountID, 50)
	if err != nil {
		return summary, nil
	}

	summary.CompletedCycles = len(history)

	// Compute history stats
	if len(history) > 0 {
		var totalDelta float64
		for _, cycle := range history {
			totalDelta += cycle.TotalDelta
			if cycle.PeakUsage > summary.PeakCycle {
				summary.PeakCycle = cycle.PeakUsage
			}
		}
		summary.AvgPerCycle = totalDelta / float64(len(history))
		summary.TotalTracked = totalDelta
	}

	// Add active cycle intelligence
	if activeCycle != nil {
		summary.TotalTracked += activeCycle.TotalDelta
		if activeCycle.PeakUsage > summary.PeakCycle {
			summary.PeakCycle = activeCycle.PeakUsage
		}
		summary.CycleSnapshots = activeCycle.SnapshotCount

		cycleAge := time.Since(activeCycle.CycleStart)
		summary.CycleAge = formatDuration(cycleAge)

		// Rate calculation (30-minute guard rail to avoid noisy extrapolation)
		if cycleAge.Minutes() >= 30 && activeCycle.TotalDelta > 0 {
			summary.CurrentRate = activeCycle.TotalDelta / cycleAge.Hours()
			summary.HasIntelligence = true

			if summary.ResetTime != nil {
				hoursLeft := time.Until(*summary.ResetTime).Hours()
				if hoursLeft > 0 {
					currentUsage := 1.0 - summary.RemainingFraction
					projected := currentUsage + summary.CurrentRate*hoursLeft
					if projected > 1.0 {
						projected = 1.0
					}
					summary.ProjectedUsage = projected

					// Calculate when quota will exhaust at current rate
					if summary.CurrentRate > 0 && summary.RemainingFraction > 0 {
						hoursToExhaust := summary.RemainingFraction / summary.CurrentRate
						exhaustTime := time.Now().Add(time.Duration(hoursToExhaust * float64(time.Hour)))
						if exhaustTime.Before(*summary.ResetTime) {
							summary.ProjectedExhaustion = &exhaustTime
						}
					}
				}
			}
		}
	}

	return summary, nil
}

// AllUsageSummaries computes intelligence for all models of an account.
func (t *Tracker) AllUsageSummaries(snap *client.Snapshot, accountID int64) ([]*UsageSummary, error) {
	if snap == nil || len(snap.Models) == 0 {
		return nil, nil
	}

	var summaries []*UsageSummary
	for _, model := range snap.Models {
		if model.ModelID == "" {
			continue
		}
		s, err := t.UsageSummaryForModel(model.ModelID, accountID, model)
		if err != nil {
			t.logger.Warn("Failed to compute summary", "model", model.ModelID, "error", err)
			continue
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// ComputeBudgetForecast calculates budget burn rate projections.
func ComputeBudgetForecast(db *store.Store) *BudgetForecast {
	budget := db.GetConfigFloat("budget_monthly", 0)
	if budget <= 0 {
		return nil
	}

	// Get current month's spend from subscriptions
	overview, err := db.SubscriptionOverview()
	if err != nil {
		return nil
	}

	now := time.Now()
	dayOfMonth := now.Day()
	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()

	forecast := &BudgetForecast{
		MonthlyBudget: budget,
		CurrentSpend:  overview.TotalMonthlySpend,
		DayOfMonth:    dayOfMonth,
		DaysInMonth:   daysInMonth,
	}

	// For fixed monthly subscriptions, burn rate is total cost / days in month
	// (not day-of-month, which would produce nonsensical rates on day 1)
	if daysInMonth > 0 {
		forecast.BurnRatePerDay = overview.TotalMonthlySpend / float64(daysInMonth)
		forecast.ProjectedMonthlySpend = overview.TotalMonthlySpend
	}

	forecast.OnTrack = forecast.ProjectedMonthlySpend <= budget

	if !forecast.OnTrack && forecast.BurnRatePerDay > 0 {
		daysUntilExhausted := int(budget / forecast.BurnRatePerDay)
		forecast.DaysUntilBudgetExhausted = &daysUntilExhausted
	}

	return forecast
}

// formatDuration formats a duration into a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}
