package agent

import (
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/claude"
)

// pollClaudeBridge reads Claude Code statusline data and stores a snapshot.
// Called alongside each Antigravity poll when the bridge is enabled.
func (a *PollingAgent) pollClaudeBridge() {
	if !a.store.GetConfigBool("claude_bridge") {
		return
	}

	if !claude.IsFresh(claude.DefaultStaleness) {
		return
	}

	rl, err := claude.ReadData()
	if err != nil {
		a.logger.Debug("Claude bridge read error", "error", err)
		return
	}
	if !claude.IsValid(rl) {
		return
	}

	// Build snapshot values
	var fiveHourPct float64
	var sevenDayPct *float64
	var fiveReset, sevenReset *time.Time

	if rl.FiveHour != nil {
		fiveHourPct = rl.FiveHour.UsedPercentage
		if rl.FiveHour.ResetsAt > 0 {
			t := time.Unix(rl.FiveHour.ResetsAt, 0).UTC()
			fiveReset = &t
		}
	}
	if rl.SevenDay != nil {
		v := rl.SevenDay.UsedPercentage
		sevenDayPct = &v
		if rl.SevenDay.ResetsAt > 0 {
			t := time.Unix(rl.SevenDay.ResetsAt, 0).UTC()
			sevenReset = &t
		}
	}

	if _, err := a.store.InsertClaudeSnapshot(fiveHourPct, sevenDayPct, fiveReset, sevenReset, "statusline", nil); err != nil {
		a.logger.Error("Failed to store Claude Code snapshot", "error", err)
		return
	}

	// Update data source bookkeeping
	a.store.UpdateSourceCapture("claude_code")

	// Check Claude notification thresholds
	if a.notifier != nil {
		a.notifier.CheckClaudeQuota("five_hour", fiveHourPct)
		if sevenDayPct != nil {
			a.notifier.CheckClaudeQuota("seven_day", *sevenDayPct)
		}
	}

	// Ensure bridge is still healthy
	claude.EnsureBridge(a.logger)

	// Feed Claude session manager
	if a.claudeSM != nil {
		vals := []float64{fiveHourPct}
		if sevenDayPct != nil {
			vals = append(vals, *sevenDayPct)
		}
		a.claudeSM.ReportPoll(vals)
	}

	a.logger.Debug("Claude Code bridge snapshot stored",
		"five_hour_pct", fiveHourPct,
		"seven_day_pct", sevenDayPct)
}
