package advisor

import (
	"testing"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
)

func TestRecommend_NoSnapshots(t *testing.T) {
	rec := Recommend(nil, nil)
	if rec.Action != "stay" {
		t.Errorf("action = %q, want %q", rec.Action, "stay")
	}
	if rec.BestAccount != nil {
		t.Error("expected no best account for empty input")
	}
}

func TestRecommend_SingleAccount(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(3 * time.Hour)

	snap := &client.Snapshot{
		AccountID:  1,
		Email:      "solo@example.com",
		PlanName:   "Pro",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.7, RemainingPercent: 70, ResetTime: &resetTime},
		},
	}

	rec := Recommend([]*client.Snapshot{snap}, nil)
	if rec.Action != "stay" {
		t.Errorf("action = %q, want %q (single account should always stay)", rec.Action, "stay")
	}
	if rec.BestAccount == nil {
		t.Fatal("expected best account to be set")
	}
	if rec.BestAccount.Email != "solo@example.com" {
		t.Errorf("best email = %q, want %q", rec.BestAccount.Email, "solo@example.com")
	}
	if len(rec.Alternatives) != 0 {
		t.Errorf("expected 0 alternatives for single account, got %d", len(rec.Alternatives))
	}
}

func TestRecommend_SwitchWhenBetterAccountExists(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(3 * time.Hour)

	current := &client.Snapshot{
		AccountID:  1,
		Email:      "depleted@example.com",
		PlanName:   "Pro",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.05, RemainingPercent: 5, ResetTime: &resetTime},
			{Label: "Gemini Pro", RemainingFraction: 0.05, RemainingPercent: 5, ResetTime: &resetTime},
		},
	}
	better := &client.Snapshot{
		AccountID:  2,
		Email:      "fresh@example.com",
		PlanName:   "Pro",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.9, RemainingPercent: 90, ResetTime: &resetTime},
			{Label: "Gemini Pro", RemainingFraction: 0.85, RemainingPercent: 85, ResetTime: &resetTime},
		},
	}

	// Current (depleted) is first — should recommend switching to fresh
	rec := Recommend([]*client.Snapshot{current, better}, nil)
	if rec.Action != "switch" {
		t.Errorf("action = %q, want %q (large score gap should trigger switch)", rec.Action, "switch")
	}
	if rec.BestAccount == nil {
		t.Fatal("expected best account to be set")
	}
	if rec.BestAccount.Email != "fresh@example.com" {
		t.Errorf("best = %q, want %q", rec.BestAccount.Email, "fresh@example.com")
	}
}

func TestRecommend_StayWhenCurrentIsBest(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(3 * time.Hour)

	current := &client.Snapshot{
		AccountID:  1,
		Email:      "current@example.com",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.8, RemainingPercent: 80, ResetTime: &resetTime},
		},
	}
	other := &client.Snapshot{
		AccountID:  2,
		Email:      "other@example.com",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.75, RemainingPercent: 75, ResetTime: &resetTime},
		},
	}

	rec := Recommend([]*client.Snapshot{current, other}, nil)
	if rec.Action != "stay" {
		t.Errorf("action = %q, want %q (current is best or close)", rec.Action, "stay")
	}
}

func TestRecommend_AllExhaustedProducesValidResult(t *testing.T) {
	now := time.Now()
	soonReset := now.Add(15 * time.Minute)

	snap1 := &client.Snapshot{
		AccountID:  1,
		Email:      "exhausted1@example.com",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.0, RemainingPercent: 0, IsExhausted: true, ResetTime: &soonReset},
			{Label: "Gemini Pro", RemainingFraction: 0.0, RemainingPercent: 0, IsExhausted: true, ResetTime: &soonReset},
		},
	}
	snap2 := &client.Snapshot{
		AccountID:  2,
		Email:      "exhausted2@example.com",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.0, RemainingPercent: 0, IsExhausted: true, ResetTime: &soonReset},
			{Label: "Gemini Pro", RemainingFraction: 0.0, RemainingPercent: 0, IsExhausted: true, ResetTime: &soonReset},
		},
	}

	rec := Recommend([]*client.Snapshot{snap1, snap2}, nil)

	// When all accounts are exhausted, advisor should still produce a valid recommendation
	validActions := map[string]bool{"stay": true, "wait": true, "switch": true}
	if !validActions[rec.Action] {
		t.Errorf("action = %q, expected one of stay/wait/switch", rec.Action)
	}
	if rec.BestAccount == nil {
		t.Error("expected BestAccount to be set even when all exhausted")
	}
	if rec.Reason == "" {
		t.Error("expected reason to be set")
	}
}

func TestRecommend_GeneratedAtIsSet(t *testing.T) {
	before := time.Now()
	rec := Recommend(nil, nil)
	after := time.Now()

	if rec.GeneratedAt.Before(before) || rec.GeneratedAt.After(after) {
		t.Errorf("generatedAt = %v, expected between %v and %v", rec.GeneratedAt, before, after)
	}
}

func TestRecommend_ReasonAlwaysSet(t *testing.T) {
	rec := Recommend(nil, nil)
	if rec.Reason == "" {
		t.Error("expected reason to be set even for empty input")
	}
}
