// Package gitcorr correlates git commit activity with AI token consumption.
//
// It parses git log output from local repositories and matches each commit's
// timestamp with Claude Code JSONL session data to compute actual per-commit
// token usage and cost — not estimated from diffs, but from real consumption.
//
// Architecture:
//   - Runs `git log` via os/exec (no git library dependency)
//   - Correlates commits with Claude Code sessions using a configurable time window
//   - Uses F5 model pricing for cost estimation
//   - Pure computation — no database writes
package gitcorr

import (
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/claude"
)

// ── Types ───────────────────────────────────────────────────────

// CommitCost represents a single git commit with correlated AI cost data.
type CommitCost struct {
	Hash         string  `json:"hash"`
	ShortHash    string  `json:"shortHash"`
	Date         string  `json:"date"`     // ISO 8601
	DateStr      string  `json:"dateStr"`  // YYYY-MM-DD
	Message      string  `json:"message"`
	Author       string  `json:"author"`
	Branch       string  `json:"branch"`   // branch name if available
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CacheTokens  int64   `json:"cacheTokens"`
	TotalTokens  int64   `json:"totalTokens"`
	CostUSD      float64 `json:"costUSD"`
	Sessions     int     `json:"sessions"` // number of correlated sessions
	Turns        int     `json:"turns"`    // number of AI turns
}

// BranchCost aggregates costs for a single branch.
type BranchCost struct {
	Name         string  `json:"name"`
	Commits      int     `json:"commits"`
	TotalTokens  int64   `json:"totalTokens"`
	CostUSD      float64 `json:"costUSD"`
	AvgPerCommit float64 `json:"avgPerCommit"` // avg cost per commit
}

// Summary is the top-level response for /api/git-costs.
type Summary struct {
	Commits  []CommitCost `json:"commits"`
	Branches []BranchCost `json:"branches"`
	Totals   Totals       `json:"totals"`
	Period   Period       `json:"period"`
	RepoPath string      `json:"repoPath"`
}

// Totals holds aggregate stats.
type Totals struct {
	CommitCount  int     `json:"commitCount"`
	TotalTokens  int64   `json:"totalTokens"`
	CostUSD      float64 `json:"costUSD"`
	AvgPerCommit float64 `json:"avgPerCommit"`
	TopBranch    string  `json:"topBranch"` // costliest branch
}

// Period describes the time range.
type Period struct {
	Start string `json:"start"` // YYYY-MM-DD
	End   string `json:"end"`   // YYYY-MM-DD
	Days  int    `json:"days"`
}

// PriceFn is a callback to look up $/1M token pricing by model ID.
type PriceFn func(modelID string) (float64, float64, float64, bool)

// ── Configuration ───────────────────────────────────────────────

// DefaultWindowMinutes is the default time window (in minutes) before a commit
// to look for Claude Code session activity.
const DefaultWindowMinutes = 30

// ── Core Function ───────────────────────────────────────────────

// Analyze correlates git commits with Claude Code token usage.
// repoPath: path to the git repository (must contain .git/)
// days: number of days to analyze (default 30)
// windowMin: time window in minutes before each commit to look for AI activity
// priceFn: model pricing callback
func Analyze(repoPath string, days, windowMin int, priceFn PriceFn) (*Summary, error) {
	if days <= 0 {
		days = 30
	}
	if windowMin <= 0 {
		windowMin = DefaultWindowMinutes
	}

	// Validate repo path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("gitcorr: invalid repo path: %w", err)
	}

	// 1. Parse git log
	commits, err := parseGitLog(absPath, days)
	if err != nil {
		return nil, fmt.Errorf("gitcorr: %w", err)
	}
	if len(commits) == 0 {
		return &Summary{
			Commits:  []CommitCost{},
			Branches: []BranchCost{},
			RepoPath: absPath,
			Period:   Period{Days: days},
		}, nil
	}

	// 2. Load all Claude Code session records
	allUsages, err := loadClaudeUsages(days)
	if err != nil {
		// Non-fatal: return commits without cost data
		return buildSummary(commits, absPath, days), nil
	}

	// 3. Correlate each commit with session data
	window := time.Duration(windowMin) * time.Minute
	correlateCommits(commits, allUsages, window, priceFn)

	return buildSummary(commits, absPath, days), nil
}

// ── Git Log Parsing ─────────────────────────────────────────────

// gitCommit is the raw parsed git commit from git log output.
type gitCommit struct {
	hash    string
	date    time.Time
	author  string
	message string
	branch  string
}

// parseGitLog runs `git log` and parses the output into structured commits.
func parseGitLog(repoPath string, days int) ([]*CommitCost, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// Format: hash|ISO-date|author|subject|decoration
	cmd := exec.Command("git", "log",
		"--all",
		"--since="+since,
		"--format=%H|%aI|%an|%s|%D",
		"--no-merges",
	)
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed (is %s a git repo?): %w", repoPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var commits []*CommitCost

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 4 {
			continue
		}

		hash := parts[0]
		dateStr := parts[1]
		author := parts[2]
		message := parts[3]
		decoration := ""
		if len(parts) >= 5 {
			decoration = parts[4]
		}

		t, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			// Try alternative formats
			t, err = time.Parse("2006-01-02T15:04:05-07:00", dateStr)
			if err != nil {
				continue
			}
		}

		branch := extractBranch(decoration)

		short := hash
		if len(hash) > 7 {
			short = hash[:7]
		}

		commits = append(commits, &CommitCost{
			Hash:      hash,
			ShortHash: short,
			Date:      t.Format(time.RFC3339),
			DateStr:   t.Format("2006-01-02"),
			Message:   truncateMsg(message, 80),
			Author:    author,
			Branch:    branch,
		})
	}

	return commits, nil
}

// ── Claude Session Correlation ──────────────────────────────────

// sessionUsage is a simplified record for time-based matching.
type sessionUsage struct {
	timestamp    time.Time
	sessionID    string
	model        string
	inputTokens  int64
	outputTokens int64
	cacheRead    int64
	cacheCreate  int64
}

// loadClaudeUsages loads all Claude Code session records for time correlation.
func loadClaudeUsages(days int) ([]sessionUsage, error) {
	files, err := claude.FindSessionFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	var all []sessionUsage

	for _, path := range files {
		usages, err := claude.ParseSessionFile(path)
		if err != nil {
			continue
		}
		for _, u := range usages {
			t, err := time.Parse(time.RFC3339Nano, u.Timestamp)
			if err != nil {
				t, err = time.Parse(time.RFC3339, u.Timestamp)
				if err != nil {
					continue
				}
			}
			if t.Before(cutoff) {
				continue
			}
			all = append(all, sessionUsage{
				timestamp:    t,
				sessionID:    u.SessionID,
				model:        u.Model,
				inputTokens:  u.InputTokens,
				outputTokens: u.OutputTokens,
				cacheRead:    u.CacheReadTokens,
				cacheCreate:  u.CacheCreateTokens,
			})
		}
	}

	// Sort by timestamp for efficient window scanning
	sort.Slice(all, func(i, j int) bool { return all[i].timestamp.Before(all[j].timestamp) })

	return all, nil
}

// correlateCommits matches each commit with Claude sessions that occurred
// within the time window before the commit.
func correlateCommits(commits []*CommitCost, usages []sessionUsage, window time.Duration, priceFn PriceFn) {
	if len(usages) == 0 {
		return
	}

	for _, c := range commits {
		commitTime, err := time.Parse(time.RFC3339, c.Date)
		if err != nil {
			continue
		}

		// Window: [commitTime - window, commitTime]
		windowStart := commitTime.Add(-window)
		windowEnd := commitTime

		sessionSet := make(map[string]bool)

		for _, u := range usages {
			if u.timestamp.Before(windowStart) {
				continue
			}
			if u.timestamp.After(windowEnd) {
				// Since usages are sorted, no more matches possible
				// (but commits aren't sorted the same way, so we can't break)
				continue
			}

			c.InputTokens += u.inputTokens
			c.OutputTokens += u.outputTokens
			c.CacheTokens += u.cacheRead + u.cacheCreate
			c.Turns++
			sessionSet[u.sessionID] = true

			// Cost calculation
			if priceFn != nil {
				inPrice, outPrice, cachePrice, found := priceFn(u.model)
				if found {
					c.CostUSD += float64(u.inputTokens) / 1_000_000 * inPrice
					c.CostUSD += float64(u.outputTokens) / 1_000_000 * outPrice
					c.CostUSD += float64(u.cacheRead+u.cacheCreate) / 1_000_000 * cachePrice
				}
			}
		}

		c.TotalTokens = c.InputTokens + c.OutputTokens
		c.CostUSD = round2(c.CostUSD)
		c.Sessions = len(sessionSet)
	}
}

// ── Summary Builder ─────────────────────────────────────────────

func buildSummary(commits []*CommitCost, repoPath string, days int) *Summary {
	s := &Summary{
		RepoPath: repoPath,
		Period:   Period{Days: days},
	}

	branchMap := make(map[string]*BranchCost)

	for _, c := range commits {
		s.Commits = append(s.Commits, *c)
		s.Totals.CommitCount++
		s.Totals.TotalTokens += c.TotalTokens
		s.Totals.CostUSD += c.CostUSD

		// Branch aggregation
		bname := c.Branch
		if bname == "" {
			bname = "(unknown)"
		}
		b, ok := branchMap[bname]
		if !ok {
			b = &BranchCost{Name: bname}
			branchMap[bname] = b
		}
		b.Commits++
		b.TotalTokens += c.TotalTokens
		b.CostUSD += c.CostUSD
	}

	s.Totals.CostUSD = round2(s.Totals.CostUSD)
	if s.Totals.CommitCount > 0 {
		s.Totals.AvgPerCommit = round2(s.Totals.CostUSD / float64(s.Totals.CommitCount))
	}

	// Build branch list sorted by cost descending
	for _, b := range branchMap {
		b.CostUSD = round2(b.CostUSD)
		if b.Commits > 0 {
			b.AvgPerCommit = round2(b.CostUSD / float64(b.Commits))
		}
		s.Branches = append(s.Branches, *b)
	}
	sort.Slice(s.Branches, func(i, j int) bool {
		return s.Branches[i].CostUSD > s.Branches[j].CostUSD
	})

	if len(s.Branches) > 0 {
		s.Totals.TopBranch = s.Branches[0].Name
	}

	// Period bounds
	if len(s.Commits) > 0 {
		s.Period.Start = s.Commits[len(s.Commits)-1].DateStr
		s.Period.End = s.Commits[0].DateStr
	}

	return s
}

// ── Helpers ─────────────────────────────────────────────────────

// extractBranch parses branch names from git log decoration.
// Example decoration: "HEAD -> main, origin/main" → "main"
func extractBranch(decoration string) string {
	decoration = strings.TrimSpace(decoration)
	if decoration == "" {
		return ""
	}

	// Look for "HEAD -> branchname" first
	if idx := strings.Index(decoration, "HEAD -> "); idx >= 0 {
		rest := decoration[idx+8:]
		if comma := strings.Index(rest, ","); comma >= 0 {
			return strings.TrimSpace(rest[:comma])
		}
		return strings.TrimSpace(rest)
	}

	// Otherwise take the first ref, strip origin/ prefix
	parts := strings.SplitN(decoration, ",", 2)
	ref := strings.TrimSpace(parts[0])
	ref = strings.TrimPrefix(ref, "origin/")
	ref = strings.TrimPrefix(ref, "tag: ")
	return ref
}

func truncateMsg(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen-3] + "..."
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
