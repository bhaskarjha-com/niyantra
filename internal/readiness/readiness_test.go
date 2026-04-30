package readiness

import (
	"testing"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
)

func TestCalculate_NilSnapshots(t *testing.T) {
	result := Calculate(nil, 0.0)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestCalculate_EmptySlice(t *testing.T) {
	result := Calculate([]*client.Snapshot{}, 0.0)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(result))
	}
}

func TestCalculate_SkipsNilEntries(t *testing.T) {
	result := Calculate([]*client.Snapshot{nil, nil}, 0.0)
	if len(result) != 0 {
		t.Errorf("expected empty result for all-nil input, got %d", len(result))
	}
}

func TestCalculate_SingleAccount(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(2 * time.Hour)

	snap := &client.Snapshot{
		AccountID:  1,
		Email:      "test@example.com",
		PlanName:   "Pro",
		CapturedAt: now.Add(-5 * time.Minute),
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.8, RemainingPercent: 80, ResetTime: &resetTime},
		},
	}

	result := Calculate([]*client.Snapshot{snap}, 0.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 account, got %d", len(result))
	}

	ar := result[0]
	if ar.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", ar.Email, "test@example.com")
	}
	if ar.PlanName != "Pro" {
		t.Errorf("plan = %q, want %q", ar.PlanName, "Pro")
	}
	if !ar.IsReady {
		t.Error("expected account to be ready with 80% remaining")
	}
	if len(ar.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(ar.Models))
	}
	if ar.Models[0].RemainingPercent != 80 {
		t.Errorf("model remaining = %f, want 80", ar.Models[0].RemainingPercent)
	}
}

func TestCalculate_ExhaustedAccount(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(1 * time.Hour)

	snap := &client.Snapshot{
		AccountID:  1,
		Email:      "exhausted@example.com",
		PlanName:   "Free",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.0, RemainingPercent: 0, IsExhausted: true, ResetTime: &resetTime},
		},
	}

	result := Calculate([]*client.Snapshot{snap}, 0.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 account, got %d", len(result))
	}

	// With threshold 0.0, even 0% remaining counts as "not ready" because IsExhausted is true
	// Actually, the readiness check is: g.RemainingFraction > threshold
	// 0.0 > 0.0 is false, so IsReady should be false
	if result[0].IsReady {
		t.Error("expected exhausted account to not be ready at threshold 0.0")
	}
}

func TestCalculate_ThresholdFiltering(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(2 * time.Hour)

	snap := &client.Snapshot{
		AccountID:  1,
		Email:      "threshold@example.com",
		PlanName:   "Pro",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.15, RemainingPercent: 15, ResetTime: &resetTime},
		},
	}

	// At threshold 0.0, 15% should be ready (0.15 > 0.0)
	result := Calculate([]*client.Snapshot{snap}, 0.0)
	if !result[0].IsReady {
		t.Error("expected ready at threshold 0.0 with 15% remaining")
	}

	// At threshold 0.2, 15% should NOT be ready (0.15 is NOT > 0.2)
	result = Calculate([]*client.Snapshot{snap}, 0.2)
	if result[0].IsReady {
		t.Error("expected NOT ready at threshold 0.2 with 15% remaining")
	}
}

func TestCalculate_SortingReadyFirst(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(2 * time.Hour)

	exhausted := &client.Snapshot{
		AccountID:  1,
		Email:      "exhausted@example.com",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.0, IsExhausted: true, ResetTime: &resetTime},
		},
	}
	ready := &client.Snapshot{
		AccountID:  2,
		Email:      "ready@example.com",
		CapturedAt: now.Add(-1 * time.Hour),
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet", RemainingFraction: 0.9, RemainingPercent: 90, ResetTime: &resetTime},
		},
	}

	// Pass exhausted first — result should sort ready first
	result := Calculate([]*client.Snapshot{exhausted, ready}, 0.0)
	if len(result) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(result))
	}
	if result[0].Email != "ready@example.com" {
		t.Errorf("expected ready account first, got %q", result[0].Email)
	}
}

func TestCalculate_MultipleModelsGrouping(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(3 * time.Hour)

	snap := &client.Snapshot{
		AccountID:  1,
		Email:      "multi@example.com",
		CapturedAt: now,
		Models: []client.ModelQuota{
			{Label: "Claude Sonnet 4.6 (Thinking)", RemainingFraction: 0.5, RemainingPercent: 50, ResetTime: &resetTime},
			{Label: "GPT-4.1", RemainingFraction: 0.6, RemainingPercent: 60, ResetTime: &resetTime},
			{Label: "Gemini 2.5 Pro", RemainingFraction: 0.8, RemainingPercent: 80, ResetTime: &resetTime},
			{Label: "Gemini 2.5 Flash", RemainingFraction: 0.9, RemainingPercent: 90, ResetTime: &resetTime},
		},
	}

	result := Calculate([]*client.Snapshot{snap}, 0.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 account, got %d", len(result))
	}
	if len(result[0].Models) != 4 {
		t.Errorf("expected 4 models, got %d", len(result[0].Models))
	}
	// Should have groups (exact count depends on GroupModels logic)
	if len(result[0].Groups) == 0 {
		t.Error("expected at least 1 group")
	}
}

func TestFormatStaleness(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{10 * time.Second, "just now"},
		{1 * time.Minute, "1 min ago"},
		{5 * time.Minute, "5 min ago"},
		{1 * time.Hour, "1 hour ago"},
		{3 * time.Hour, "3h ago"},
		{24 * time.Hour, "1 day ago"},
		{72 * time.Hour, "3d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatStaleness(tt.duration)
			if got != tt.want {
				t.Errorf("formatStaleness(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

// TestStaleSnapshotInfersReset verifies that a snapshot from 10 days ago
// with 0% remaining and reset time in the past shows RemainingPercent=100
// and StalenessLabel="Stale". This is the C3 regression test.
func TestStaleSnapshotInfersReset(t *testing.T) {
	pastReset := time.Now().Add(-10 * 24 * time.Hour).Add(5 * time.Hour) // 10 days ago + 5h
	snap := &client.Snapshot{
		AccountID:  1,
		Email:      "stale@example.com",
		PlanName:   "Pro",
		CapturedAt: time.Now().Add(-10 * 24 * time.Hour), // 10 days ago
		Models: []client.ModelQuota{
			{
				ModelID:           "model-1",
				Label:             "Claude Sonnet",
				RemainingFraction: 0.0,
				RemainingPercent:  0,
				IsExhausted:       true,
				ResetTime:         &pastReset,
			},
		},
	}

	result := Calculate([]*client.Snapshot{snap}, 0.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 account, got %d", len(result))
	}

	ar := result[0]
	if ar.StalenessLabel != "Stale" {
		t.Errorf("staleness label = %q, want %q", ar.StalenessLabel, "Stale")
	}
	if len(ar.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(ar.Models))
	}
	if ar.Models[0].RemainingPercent != 100 {
		t.Errorf("stale model remaining = %f, want 100 (inferred reset)", ar.Models[0].RemainingPercent)
	}
	if ar.Models[0].IsExhausted {
		t.Error("stale model should NOT be marked exhausted after inferred reset")
	}
}

// TestFreshSnapshotUnchanged verifies that a fresh snapshot (5 min ago)
// with 0% remaining stays at 0% — it's genuinely exhausted, not stale.
func TestFreshSnapshotUnchanged(t *testing.T) {
	futureReset := time.Now().Add(2 * time.Hour) // reset in 2 hours
	snap := &client.Snapshot{
		AccountID:  1,
		Email:      "fresh@example.com",
		PlanName:   "Pro",
		CapturedAt: time.Now().Add(-5 * time.Minute), // 5 min ago
		Models: []client.ModelQuota{
			{
				ModelID:           "model-1",
				Label:             "Claude Sonnet",
				RemainingFraction: 0.0,
				RemainingPercent:  0,
				IsExhausted:       true,
				ResetTime:         &futureReset,
			},
		},
	}

	result := Calculate([]*client.Snapshot{snap}, 0.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 account, got %d", len(result))
	}

	ar := result[0]
	if ar.StalenessLabel == "Stale" {
		t.Error("fresh snapshot should NOT be labeled Stale")
	}
	if len(ar.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(ar.Models))
	}
	if ar.Models[0].RemainingPercent != 0 {
		t.Errorf("fresh model remaining = %f, want 0 (genuinely exhausted)", ar.Models[0].RemainingPercent)
	}
	if !ar.Models[0].IsExhausted {
		t.Error("fresh model should be marked exhausted")
	}
}
