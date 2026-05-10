package readiness

import (
	"fmt"
	"sort"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
)

// AccountReadiness represents the readiness state of a single account.
type AccountReadiness struct {
	AccountID        int64             `json:"accountId"`
	LatestSnapshotID int64             `json:"latestSnapshotId"`
	Email            string            `json:"email"`
	PlanName         string            `json:"planName"`
	Notes            string            `json:"notes"`
	Tags             string            `json:"tags"`
	PinnedGroup      string            `json:"pinnedGroup"`
	CreditRenewalDay int               `json:"creditRenewalDay"`
	LastSeen         time.Time         `json:"lastSeen"`
	Staleness        time.Duration     `json:"-"`
	StalenessLabel   string            `json:"stalenessLabel"`
	IsReady          bool              `json:"isReady"`
	Groups           []GroupReadiness  `json:"groups"`
	Models           []ModelDetail     `json:"models"`
	PromptCredits    float64           `json:"promptCredits"`
	MonthlyCredits   int               `json:"monthlyCredits"`
	AICredits        []client.AICredit `json:"aiCredits"`
}

// ModelDetail is a per-model quota entry for the dashboard.
type ModelDetail struct {
	ModelID          string  `json:"modelId"`
	Label            string  `json:"label"`
	RemainingPercent float64 `json:"remainingPercent"`
	IsExhausted      bool    `json:"isExhausted"`
	ResetSeconds     float64 `json:"resetSeconds"`
	GroupKey         string  `json:"groupKey"`
}

// GroupReadiness represents the readiness state of a single quota group.
type GroupReadiness struct {
	GroupKey          string     `json:"groupKey"`
	DisplayName       string     `json:"displayName"`
	RemainingPercent  float64    `json:"remainingPercent"`
	IsExhausted       bool       `json:"isExhausted"`
	IsReady           bool       `json:"isReady"`
	Color             string     `json:"color"`
	ResetTime         *time.Time `json:"resetTime,omitempty"`
	TimeUntilResetSec float64    `json:"timeUntilResetSec"`
}

// Calculate computes readiness for all accounts from their latest snapshots.
// threshold is the minimum remainingFraction to consider "ready" (0.0 = any remaining).
func Calculate(snapshots []*client.Snapshot, threshold float64) []AccountReadiness {
	var results []AccountReadiness

	stalenessThreshold := 6 * time.Hour

	for _, snap := range snapshots {
		if snap == nil {
			continue
		}

		staleness := time.Since(snap.CapturedAt)
		isStale := staleness > stalenessThreshold

		ar := AccountReadiness{
			AccountID:        snap.AccountID,
			LatestSnapshotID: snap.ID,
			Email:            snap.Email,
			PlanName:         snap.PlanName,
			LastSeen:         snap.CapturedAt,
			Staleness:        staleness,
			StalenessLabel:   formatStaleness(staleness),
			IsReady:          true,
			PromptCredits:    snap.PromptCredits,
			MonthlyCredits:   snap.MonthlyCredits,
			AICredits:        snap.AICredits,
		}

		// Q3: Always show actual time-ago — the time IS the staleness indicator
		// Visual dimming in the UI handles the "stale" signal instead

		// Per-model details + build corrected models for group computation
		// Bug fix: GroupModels must use reset-time-corrected values, not raw snapshot data.
		// Without this, group-level columns (Claude+GPT) show stale values even after reset.
		correctedModels := make([]client.ModelQuota, 0, len(snap.Models))
		for _, m := range snap.Models {
			resetSec := 0.0
			remainingPct := m.RemainingPercent
			remainingFrac := m.RemainingFraction
			exhausted := m.IsExhausted

			if m.ResetTime != nil {
				resetSec = time.Until(*m.ResetTime).Seconds()
				if resetSec < 0 {
					resetSec = 0
					// C3: If snapshot is stale AND reset time has passed,
					// infer quota has refilled (rolling 5h resets)
					if isStale {
						remainingPct = 100
						remainingFrac = 1.0
						exhausted = false
					}
				}
			}
			ar.Models = append(ar.Models, ModelDetail{
				ModelID:          m.ModelID,
				Label:            m.Label,
				RemainingPercent: remainingPct,
				IsExhausted:      exhausted,
				ResetSeconds:     resetSec,
				GroupKey:         client.GroupForModel(m.ModelID, m.Label),
			})

			// Build corrected ModelQuota for group computation
			correctedModels = append(correctedModels, client.ModelQuota{
				ModelID:           m.ModelID,
				Label:             m.Label,
				RemainingFraction: remainingFrac,
				RemainingPercent:  remainingPct,
				IsExhausted:       exhausted,
				ResetTime:         m.ResetTime,
				TimeUntilReset:    m.TimeUntilReset,
			})
		}

		// Group models using CORRECTED values (not raw snapshot)
		groups := client.GroupModels(correctedModels)
		for _, g := range groups {
			gr := GroupReadiness{
				GroupKey:         g.GroupKey,
				DisplayName:      g.DisplayName,
				RemainingPercent: g.RemainingFraction * 100,
				IsExhausted:      g.IsExhausted,
				Color:            g.Color,
				ResetTime:        g.ResetTime,
			}

			if g.ResetTime != nil {
				sec := time.Until(*g.ResetTime).Seconds()
				if sec < 0 {
					sec = 0
				}
				gr.TimeUntilResetSec = sec
			}

			gr.IsReady = g.RemainingFraction > threshold
			if !gr.IsReady {
				ar.IsReady = false
			}

			ar.Groups = append(ar.Groups, gr)
		}

		results = append(results, ar)
	}

	// Sort: ready accounts first, then by most recently seen
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsReady != results[j].IsReady {
			return results[i].IsReady
		}
		return results[i].LastSeen.After(results[j].LastSeen)
	})

	return results
}

// formatStaleness returns a human-readable staleness label.
func formatStaleness(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
