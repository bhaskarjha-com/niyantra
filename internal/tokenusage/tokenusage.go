// Package tokenusage provides unified token usage analytics across all
// AI coding providers tracked by Niyantra. It aggregates data from:
//
//   - Claude Code local JSONL sessions (full input/output/cache tokens)
//   - Antigravity quota delta tracking (estimated via snapshot fractions)
//   - Codex/ChatGPT quota snapshots (estimated usage)
//   - Cursor quota snapshots (estimated usage)
//   - Gemini CLI quota snapshots (estimated usage)
//
// Architecture:
//   - Pure computation — no side effects, no database writes
//   - Consumes data from store and claude packages
//   - Returns structured analytics for API and MCP consumption
package tokenusage

import (
	"math"
	"sort"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/claude"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// ── Types ───────────────────────────────────────────────────────

// Summary is the top-level analytics response for /api/token-usage.
type Summary struct {
	Totals   Totals           `json:"totals"`
	ByModel  []ModelBreakdown `json:"byModel"`
	ByDay    []DailyBreakdown `json:"byDay"`
	KPIs     KPIs             `json:"kpis"`
	Period   Period           `json:"period"`
	Provider string           `json:"provider"` // "all" or specific provider
}

// Totals holds aggregate token counts.
type Totals struct {
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CacheTokens  int64   `json:"cacheTokens"`
	TotalTokens  int64   `json:"totalTokens"`
	EstCostUSD   float64 `json:"estimatedCostUSD"`
	Turns        int     `json:"turns"`
	Sessions     int     `json:"sessions"`
}

// ModelBreakdown aggregates usage per model.
type ModelBreakdown struct {
	Model        string  `json:"model"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CacheTokens  int64   `json:"cacheTokens"`
	TotalTokens  int64   `json:"totalTokens"`
	CostUSD      float64 `json:"costUSD"`
	Turns        int     `json:"turns"`
	Percentage   float64 `json:"percentage"` // % of total tokens
}

// DailyBreakdown aggregates usage per day.
type DailyBreakdown struct {
	Date         string  `json:"date"` // YYYY-MM-DD
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CacheTokens  int64   `json:"cacheTokens"`
	TotalTokens  int64   `json:"totalTokens"`
	CostUSD      float64 `json:"costUSD"`
	Sessions     int     `json:"sessions"`
}

// KPIs holds computed key performance indicators.
type KPIs struct {
	DaysActive      int     `json:"daysActive"`
	AvgTokensPerDay int64   `json:"avgTokensPerDay"`
	AvgCostPerDay   float64 `json:"avgCostPerDay"`
	CacheHitRate    float64 `json:"cacheHitRate"` // 0.0–1.0
	TopModel        string  `json:"topModel"`
	PeakDay         string  `json:"peakDay"`      // YYYY-MM-DD with most tokens
	PeakDayTokens   int64   `json:"peakDayTokens"`
}

// Period describes the time range of the analytics.
type Period struct {
	Start string `json:"start"` // YYYY-MM-DD
	End   string `json:"end"`   // YYYY-MM-DD
	Days  int    `json:"days"`
}

// PriceFn is a callback to look up $/1M token pricing by model ID.
// Returns (inputPer1M, outputPer1M, cachePer1M, found).
type PriceFn func(modelID string) (float64, float64, float64, bool)

// ── Aggregation ─────────────────────────────────────────────────

// AggregateFromClaude builds a Summary from Claude Code's local JSONL data.
// This is the primary data source since it has full per-turn token granularity.
func AggregateFromClaude(days int, priceFn PriceFn) (*Summary, error) {
	if days <= 0 {
		days = 30
	}

	// Delegate to the existing Claude deep parser
	claudePriceFn := func(modelID string) (float64, float64, float64, bool) {
		if priceFn == nil {
			return 0, 0, 0, false
		}
		return priceFn(modelID)
	}

	usage, err := claude.AggregateUsage(days, claudePriceFn)
	if err != nil {
		return nil, err
	}
	if usage == nil {
		return emptySummary(days), nil
	}

	// Convert claude.UsageSummary → tokenusage.Summary
	s := &Summary{
		Provider: "claude",
		Totals: Totals{
			InputTokens:  usage.TotalInput,
			OutputTokens: usage.TotalOutput,
			TotalTokens:  usage.TotalTokens,
			EstCostUSD:   round2(usage.TotalCost),
			Sessions:     usage.TotalSessions,
		},
		Period: Period{Days: days},
	}

	// byDay
	for _, d := range usage.Days {
		db := DailyBreakdown{
			Date:         d.Date,
			InputTokens:  d.InputTokens,
			OutputTokens: d.OutputTokens,
			CacheTokens:  d.CacheRead + d.CacheCreate,
			TotalTokens:  d.InputTokens + d.OutputTokens,
			CostUSD:      round2(d.CostUSD),
			Sessions:     d.SessionCount,
		}
		s.ByDay = append(s.ByDay, db)
		s.Totals.CacheTokens += db.CacheTokens

		// Per-model breakdown accumulation
		for _, mu := range d.ByModel {
			s.mergeModel(mu)
		}
	}

	// Compute percentages and sort models
	s.computeModelPercentages()

	// Compute KPIs
	s.computeKPIs()

	// Set period bounds
	if len(s.ByDay) > 0 {
		s.Period.Start = s.ByDay[0].Date
		s.Period.End = s.ByDay[len(s.ByDay)-1].Date
	}

	return s, nil
}

// AggregateFromStore builds estimated usage from persisted token_usage rows.
// This is used for providers without direct token-level data.
func AggregateFromStore(st *store.Store, days int, provider string) (*Summary, error) {
	if days <= 0 {
		days = 30
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := st.QueryTokenUsage(cutoff, provider)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return emptySummary(days), nil
	}

	s := &Summary{
		Provider: provider,
		Period:   Period{Days: days},
	}

	dayMap := make(map[string]*DailyBreakdown)
	modelMap := make(map[string]*ModelBreakdown)

	for _, r := range rows {
		// Accumulate totals
		s.Totals.InputTokens += r.InputTokens
		s.Totals.OutputTokens += r.OutputTokens
		s.Totals.CacheTokens += r.CacheRead + r.CacheCreate
		s.Totals.EstCostUSD += r.EstimatedCost
		s.Totals.Turns += r.TurnCount
		s.Totals.Sessions += r.SessionCount

		// Daily breakdown
		d, ok := dayMap[r.Date]
		if !ok {
			d = &DailyBreakdown{Date: r.Date}
			dayMap[r.Date] = d
		}
		d.InputTokens += r.InputTokens
		d.OutputTokens += r.OutputTokens
		d.CacheTokens += r.CacheRead + r.CacheCreate
		d.TotalTokens += r.InputTokens + r.OutputTokens
		d.CostUSD += r.EstimatedCost
		d.Sessions += r.SessionCount

		// Model breakdown
		if r.Model != "" {
			m, ok := modelMap[r.Model]
			if !ok {
				m = &ModelBreakdown{Model: r.Model}
				modelMap[r.Model] = m
			}
			m.InputTokens += r.InputTokens
			m.OutputTokens += r.OutputTokens
			m.CacheTokens += r.CacheRead + r.CacheCreate
			m.TotalTokens += r.InputTokens + r.OutputTokens
			m.CostUSD += r.EstimatedCost
			m.Turns += r.TurnCount
		}
	}

	// Convert maps to sorted slices
	s.Totals.TotalTokens = s.Totals.InputTokens + s.Totals.OutputTokens
	s.Totals.EstCostUSD = round2(s.Totals.EstCostUSD)

	for _, d := range dayMap {
		d.CostUSD = round2(d.CostUSD)
		s.ByDay = append(s.ByDay, *d)
	}
	sort.Slice(s.ByDay, func(i, j int) bool { return s.ByDay[i].Date < s.ByDay[j].Date })

	for _, m := range modelMap {
		m.CostUSD = round2(m.CostUSD)
		s.ByModel = append(s.ByModel, *m)
	}

	s.computeModelPercentages()
	s.computeKPIs()

	if len(s.ByDay) > 0 {
		s.Period.Start = s.ByDay[0].Date
		s.Period.End = s.ByDay[len(s.ByDay)-1].Date
	}

	return s, nil
}

// Merge merges two Summaries into one unified view. Used when
// combining Claude (granular) data with estimated provider data.
func Merge(a, b *Summary) *Summary {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	merged := &Summary{
		Provider: "all",
		Totals: Totals{
			InputTokens:  a.Totals.InputTokens + b.Totals.InputTokens,
			OutputTokens: a.Totals.OutputTokens + b.Totals.OutputTokens,
			CacheTokens:  a.Totals.CacheTokens + b.Totals.CacheTokens,
			TotalTokens:  a.Totals.TotalTokens + b.Totals.TotalTokens,
			EstCostUSD:   round2(a.Totals.EstCostUSD + b.Totals.EstCostUSD),
			Turns:        a.Totals.Turns + b.Totals.Turns,
			Sessions:     a.Totals.Sessions + b.Totals.Sessions,
		},
		Period: Period{
			Days: a.Period.Days,
		},
	}
	if b.Period.Days > merged.Period.Days {
		merged.Period.Days = b.Period.Days
	}

	// Merge daily breakdowns
	dayMap := make(map[string]*DailyBreakdown)
	for _, d := range a.ByDay {
		dc := d
		dayMap[d.Date] = &dc
	}
	for _, d := range b.ByDay {
		if existing, ok := dayMap[d.Date]; ok {
			existing.InputTokens += d.InputTokens
			existing.OutputTokens += d.OutputTokens
			existing.CacheTokens += d.CacheTokens
			existing.TotalTokens += d.TotalTokens
			existing.CostUSD = round2(existing.CostUSD + d.CostUSD)
			existing.Sessions += d.Sessions
		} else {
			dc := d
			dayMap[d.Date] = &dc
		}
	}
	for _, d := range dayMap {
		merged.ByDay = append(merged.ByDay, *d)
	}
	sort.Slice(merged.ByDay, func(i, j int) bool { return merged.ByDay[i].Date < merged.ByDay[j].Date })

	// Merge model breakdowns
	modelMap := make(map[string]*ModelBreakdown)
	for _, m := range a.ByModel {
		mc := m
		modelMap[m.Model] = &mc
	}
	for _, m := range b.ByModel {
		if existing, ok := modelMap[m.Model]; ok {
			existing.InputTokens += m.InputTokens
			existing.OutputTokens += m.OutputTokens
			existing.CacheTokens += m.CacheTokens
			existing.TotalTokens += m.TotalTokens
			existing.CostUSD = round2(existing.CostUSD + m.CostUSD)
			existing.Turns += m.Turns
		} else {
			mc := m
			modelMap[m.Model] = &mc
		}
	}
	for _, m := range modelMap {
		merged.ByModel = append(merged.ByModel, *m)
	}

	merged.computeModelPercentages()
	merged.computeKPIs()

	if len(merged.ByDay) > 0 {
		merged.Period.Start = merged.ByDay[0].Date
		merged.Period.End = merged.ByDay[len(merged.ByDay)-1].Date
	}

	return merged
}

// ── Internal helpers ────────────────────────────────────────────

func (s *Summary) mergeModel(mu claude.ModelUsage) {
	for i := range s.ByModel {
		if s.ByModel[i].Model == mu.Model {
			s.ByModel[i].InputTokens += mu.InputTokens
			s.ByModel[i].OutputTokens += mu.OutputTokens
			s.ByModel[i].CacheTokens += mu.CacheRead + mu.CacheCreate
			s.ByModel[i].TotalTokens += mu.InputTokens + mu.OutputTokens
			s.ByModel[i].CostUSD = round2(s.ByModel[i].CostUSD + mu.CostUSD)
			s.ByModel[i].Turns += mu.Turns
			return
		}
	}
	s.ByModel = append(s.ByModel, ModelBreakdown{
		Model:        mu.Model,
		InputTokens:  mu.InputTokens,
		OutputTokens: mu.OutputTokens,
		CacheTokens:  mu.CacheRead + mu.CacheCreate,
		TotalTokens:  mu.InputTokens + mu.OutputTokens,
		CostUSD:      round2(mu.CostUSD),
		Turns:        mu.Turns,
	})
}

func (s *Summary) computeModelPercentages() {
	if s.Totals.TotalTokens == 0 {
		return
	}
	for i := range s.ByModel {
		s.ByModel[i].Percentage = round2(float64(s.ByModel[i].TotalTokens) / float64(s.Totals.TotalTokens) * 100)
	}
	// Sort by total tokens descending
	sort.Slice(s.ByModel, func(i, j int) bool {
		return s.ByModel[i].TotalTokens > s.ByModel[j].TotalTokens
	})
}

func (s *Summary) computeKPIs() {
	s.KPIs.DaysActive = len(s.ByDay)
	if s.KPIs.DaysActive > 0 {
		s.KPIs.AvgTokensPerDay = s.Totals.TotalTokens / int64(s.KPIs.DaysActive)
		s.KPIs.AvgCostPerDay = round2(s.Totals.EstCostUSD / float64(s.KPIs.DaysActive))
	}

	// Cache hit rate
	totalCache := s.Totals.CacheTokens
	totalInput := s.Totals.InputTokens
	if totalInput > 0 {
		s.KPIs.CacheHitRate = round2(float64(totalCache) / float64(totalInput+totalCache))
	}

	// Top model
	if len(s.ByModel) > 0 {
		s.KPIs.TopModel = s.ByModel[0].Model
	}

	// Peak day
	for _, d := range s.ByDay {
		if d.TotalTokens > s.KPIs.PeakDayTokens {
			s.KPIs.PeakDayTokens = d.TotalTokens
			s.KPIs.PeakDay = d.Date
		}
	}
}

func emptySummary(days int) *Summary {
	return &Summary{
		ByModel: []ModelBreakdown{},
		ByDay:   []DailyBreakdown{},
		Period:  Period{Days: days},
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
