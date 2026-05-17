package gitcorr

import (
	"testing"
	"time"
)

// ── extractBranch tests ─────────────────────────────────────────

func TestExtractBranch_HeadArrow(t *testing.T) {
	tests := []struct {
		name       string
		decoration string
		want       string
	}{
		{"HEAD arrow", "HEAD -> main, origin/main", "main"},
		{"HEAD arrow no comma", "HEAD -> feature/foo", "feature/foo"},
		{"empty", "", ""},
		{"origin only", "origin/main", "main"},
		{"tag", "tag: v1.0.0", "v1.0.0"},
		{"multiple refs", "origin/main, origin/feature", "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBranch(tt.decoration)
			if got != tt.want {
				t.Errorf("extractBranch(%q) = %q, want %q", tt.decoration, got, tt.want)
			}
		})
	}
}

// ── truncateMsg tests ───────────────────────────────────────────

func TestTruncateMsg(t *testing.T) {
	tests := []struct {
		msg    string
		maxLen int
		want   string
	}{
		{"short", 80, "short"},
		{"exactly 10 c", 12, "exactly 10 c"},
		{"this message is definitely way too long to fit in the allowed limit of characters", 40, "this message is definitely way too lo..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.msg[:min(len(tt.msg), 20)], func(t *testing.T) {
			got := truncateMsg(tt.msg, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateMsg(%q, %d) = %q, want %q", tt.msg, tt.maxLen, got, tt.want)
			}
			if len(got) > tt.maxLen {
				t.Errorf("truncateMsg result len %d exceeds maxLen %d", len(got), tt.maxLen)
			}
		})
	}
}

// ── round2 tests ────────────────────────────────────────────────

func TestRound2(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{0.0, 0.0},
		{99.999, 100.0},
		{0.001, 0.0},
	}
	for _, tt := range tests {
		got := round2(tt.in)
		if got != tt.want {
			t.Errorf("round2(%f) = %f, want %f", tt.in, got, tt.want)
		}
	}
}

// ── buildSummary tests ──────────────────────────────────────────

func TestBuildSummary_Empty(t *testing.T) {
	s := buildSummary(nil, "/test/repo", 30)
	if s.Totals.CommitCount != 0 {
		t.Errorf("expected 0 commits, got %d", s.Totals.CommitCount)
	}
	if s.RepoPath != "/test/repo" {
		t.Errorf("expected /test/repo, got %s", s.RepoPath)
	}
	if s.Period.Days != 30 {
		t.Errorf("expected 30 days, got %d", s.Period.Days)
	}
}

func TestBuildSummary_WithCommits(t *testing.T) {
	commits := []*CommitCost{
		{
			Hash: "abc123", ShortHash: "abc1234", Date: "2026-05-17T10:00:00Z",
			DateStr: "2026-05-17", Message: "feat: first", Author: "dev",
			Branch: "main", TotalTokens: 1000, CostUSD: 0.50,
		},
		{
			Hash: "def456", ShortHash: "def4567", Date: "2026-05-16T09:00:00Z",
			DateStr: "2026-05-16", Message: "feat: second", Author: "dev",
			Branch: "feature/x", TotalTokens: 2000, CostUSD: 1.00,
		},
		{
			Hash: "ghi789", ShortHash: "ghi7890", Date: "2026-05-15T08:00:00Z",
			DateStr: "2026-05-15", Message: "fix: third", Author: "dev",
			Branch: "feature/x", TotalTokens: 500, CostUSD: 0.25,
		},
	}

	s := buildSummary(commits, "/repos/niyantra", 30)

	if s.Totals.CommitCount != 3 {
		t.Errorf("expected 3 commits, got %d", s.Totals.CommitCount)
	}
	if s.Totals.TotalTokens != 3500 {
		t.Errorf("expected 3500 total tokens, got %d", s.Totals.TotalTokens)
	}
	if s.Totals.CostUSD != 1.75 {
		t.Errorf("expected $1.75 total cost, got $%.2f", s.Totals.CostUSD)
	}
	if s.Totals.AvgPerCommit != 0.58 {
		t.Errorf("expected $0.58 avg, got $%.2f", s.Totals.AvgPerCommit)
	}

	// Branch aggregation
	if len(s.Branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(s.Branches))
	}
	// Sorted by cost desc — feature/x ($1.25) should be first
	if s.Branches[0].Name != "feature/x" {
		t.Errorf("expected top branch feature/x, got %s", s.Branches[0].Name)
	}
	if s.Branches[0].Commits != 2 {
		t.Errorf("expected 2 commits on feature/x, got %d", s.Branches[0].Commits)
	}
	if s.Totals.TopBranch != "feature/x" {
		t.Errorf("expected top branch feature/x, got %s", s.Totals.TopBranch)
	}

	// Period bounds
	if s.Period.Start != "2026-05-15" || s.Period.End != "2026-05-17" {
		t.Errorf("unexpected period: %s to %s", s.Period.Start, s.Period.End)
	}
}

func TestBuildSummary_UnknownBranch(t *testing.T) {
	commits := []*CommitCost{
		{Hash: "a", ShortHash: "a", Date: "2026-01-01T00:00:00Z", DateStr: "2026-01-01", Branch: ""},
	}
	s := buildSummary(commits, "/repo", 7)
	if len(s.Branches) != 1 || s.Branches[0].Name != "(unknown)" {
		t.Errorf("expected (unknown) branch, got %v", s.Branches)
	}
}

// ── correlateCommits tests ──────────────────────────────────────

func TestCorrelateCommits_EmptyUsages(t *testing.T) {
	commits := []*CommitCost{
		{Hash: "a", Date: "2026-05-17T10:00:00Z"},
	}
	correlateCommits(commits, nil, 30*time.Minute, nil)
	if commits[0].TotalTokens != 0 {
		t.Errorf("expected 0 tokens with no usages")
	}
}

func TestCorrelateCommits_WindowMatch(t *testing.T) {
	commitTime, _ := time.Parse(time.RFC3339, "2026-05-17T10:00:00Z")
	commits := []*CommitCost{
		{Hash: "abc", Date: "2026-05-17T10:00:00Z"},
	}

	usages := []sessionUsage{
		{
			timestamp:    commitTime.Add(-10 * time.Minute), // Within 30min window
			sessionID:    "sess-1",
			model:        "claude-3.5-sonnet",
			inputTokens:  500,
			outputTokens: 200,
		},
		{
			timestamp:    commitTime.Add(-45 * time.Minute), // Outside window
			sessionID:    "sess-2",
			model:        "claude-3.5-sonnet",
			inputTokens:  1000,
			outputTokens: 400,
		},
	}

	priceFn := func(model string) (float64, float64, float64, bool) {
		return 3.0, 15.0, 0.3, true // $3/M in, $15/M out, $0.3/M cache
	}

	correlateCommits(commits, usages, 30*time.Minute, priceFn)

	if commits[0].InputTokens != 500 {
		t.Errorf("expected 500 input tokens, got %d", commits[0].InputTokens)
	}
	if commits[0].OutputTokens != 200 {
		t.Errorf("expected 200 output tokens, got %d", commits[0].OutputTokens)
	}
	if commits[0].Sessions != 1 {
		t.Errorf("expected 1 session, got %d", commits[0].Sessions)
	}
	if commits[0].Turns != 1 {
		t.Errorf("expected 1 turn, got %d", commits[0].Turns)
	}
	// Cost should be calculated: (500/1M)*3 + (200/1M)*15 = 0.0015 + 0.003 = 0.0045 → rounds to 0.0
	// With small token counts the cost rounds to 0 — check tokens and sessions instead
	if commits[0].TotalTokens != 700 {
		t.Errorf("expected 700 total tokens, got %d", commits[0].TotalTokens)
	}
}

func TestCorrelateCommits_NoPriceFn(t *testing.T) {
	commitTime, _ := time.Parse(time.RFC3339, "2026-05-17T10:00:00Z")
	commits := []*CommitCost{
		{Hash: "abc", Date: "2026-05-17T10:00:00Z"},
	}
	usages := []sessionUsage{
		{
			timestamp:    commitTime.Add(-5 * time.Minute),
			sessionID:    "s1",
			model:        "claude-3.5-sonnet",
			inputTokens:  100,
			outputTokens: 50,
		},
	}

	correlateCommits(commits, usages, 30*time.Minute, nil)

	if commits[0].TotalTokens != 150 {
		t.Errorf("expected 150 total tokens, got %d", commits[0].TotalTokens)
	}
	if commits[0].CostUSD != 0 {
		t.Errorf("expected 0 cost without price fn, got %.2f", commits[0].CostUSD)
	}
}

// ── CommitCost type tests ───────────────────────────────────────

func TestCommitCostDefaults(t *testing.T) {
	c := CommitCost{}
	if c.Hash != "" || c.InputTokens != 0 || c.CostUSD != 0 {
		t.Error("expected zero-value CommitCost")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
