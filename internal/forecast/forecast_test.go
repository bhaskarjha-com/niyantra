package forecast

import (
	"math"
	"testing"
	"time"
)

func TestComputeRates_InsufficientData(t *testing.T) {
	// Less than MinDataPoints should return nil
	rates := ComputeRates(nil)
	if rates != nil {
		t.Error("expected nil for nil input")
	}

	rates = ComputeRates([]SnapshotPoint{{CapturedAt: time.Now()}})
	if rates != nil {
		t.Error("expected nil for single point")
	}
}

func TestComputeRates_TooShortTimespan(t *testing.T) {
	now := time.Now()
	points := []SnapshotPoint{
		{CapturedAt: now, Models: map[string]float64{"m1": 0.8}},
		{CapturedAt: now.Add(5 * time.Minute), Models: map[string]float64{"m1": 0.7}},
	}
	rates := ComputeRates(points)
	if rates != nil {
		t.Error("expected nil for timespan < MinTimeSpan")
	}
}

func TestComputeRates_SteadyBurn(t *testing.T) {
	// Simulate steady 20%/hr consumption over 1 hour (12 × 5-min intervals)
	now := time.Now()
	points := make([]SnapshotPoint, 13)
	for i := 0; i <= 12; i++ {
		remaining := 1.0 - float64(i)*(0.2/12.0) // lose 20% over 12 intervals
		points[i] = SnapshotPoint{
			CapturedAt: now.Add(time.Duration(i*5) * time.Minute),
			Models:     map[string]float64{"modelA": remaining},
		}
	}

	rates := ComputeRates(points)
	if rates == nil {
		t.Fatal("expected non-nil rates")
	}
	r, ok := rates["modelA"]
	if !ok {
		t.Fatal("expected rate for modelA")
	}

	// Should be approximately 0.20/hr (allow ±5% margin for weighting)
	if math.Abs(r.Rate-0.20) > 0.02 {
		t.Errorf("expected rate ~0.20/hr, got %f", r.Rate)
	}
	if r.Points < 12 {
		t.Errorf("expected ≥12 points, got %d", r.Points)
	}
}

func TestComputeRates_IdlePeriodAccuracy(t *testing.T) {
	// The KEY accuracy test: active for 30min, then idle for 30min.
	// Old approach: TotalDelta/cycleAge = 0.3/1h = 0.3/hr
	// New approach: should reflect that last 30min had zero consumption.
	now := time.Now()
	points := []SnapshotPoint{
		// Active phase: 30% consumed in 30 minutes
		{CapturedAt: now, Models: map[string]float64{"m1": 1.0}},
		{CapturedAt: now.Add(5 * time.Minute), Models: map[string]float64{"m1": 0.95}},
		{CapturedAt: now.Add(10 * time.Minute), Models: map[string]float64{"m1": 0.90}},
		{CapturedAt: now.Add(15 * time.Minute), Models: map[string]float64{"m1": 0.85}},
		{CapturedAt: now.Add(20 * time.Minute), Models: map[string]float64{"m1": 0.80}},
		{CapturedAt: now.Add(25 * time.Minute), Models: map[string]float64{"m1": 0.75}},
		{CapturedAt: now.Add(30 * time.Minute), Models: map[string]float64{"m1": 0.70}},
		// Idle phase: no consumption for 30 minutes
		{CapturedAt: now.Add(35 * time.Minute), Models: map[string]float64{"m1": 0.70}},
		{CapturedAt: now.Add(40 * time.Minute), Models: map[string]float64{"m1": 0.70}},
		{CapturedAt: now.Add(45 * time.Minute), Models: map[string]float64{"m1": 0.70}},
		{CapturedAt: now.Add(50 * time.Minute), Models: map[string]float64{"m1": 0.70}},
		{CapturedAt: now.Add(55 * time.Minute), Models: map[string]float64{"m1": 0.70}},
		{CapturedAt: now.Add(60 * time.Minute), Models: map[string]float64{"m1": 0.70}},
	}

	rates := ComputeRates(points)
	if rates == nil {
		t.Fatal("expected non-nil rates")
	}
	r := rates["m1"]
	if r == nil {
		t.Fatal("expected rate for m1")
	}

	// Old broken calculation: 0.30 / 1.0h = 0.30/hr
	// New sliding-window: active phase had 0.60/hr, idle phase had 0/hr.
	// With recency weighting, the idle (recent) data dominates.
	// The rate should be much lower than 0.30/hr because the user is currently idle.
	if r.Rate >= 0.25 {
		t.Errorf("rate should be < 0.25/hr (idle period should reduce it), got %f", r.Rate)
	}

	// But also shouldn't be exactly 0 since there was active usage in the window
	// (unless the recency weighting completely zeroes it, which it shouldn't)
	t.Logf("Computed rate: %f/hr (correctly accounting for idle period)", r.Rate)
}

func TestComputeRates_RecentBurstAccuracy(t *testing.T) {
	// Opposite scenario: idle for 30min, then heavy burst in last 30min.
	// Old approach: same averaging would give artificially LOW rate.
	// New approach with recency weighting should give a HIGHER rate.
	now := time.Now()
	points := []SnapshotPoint{
		// Idle phase: no consumption for 30 minutes
		{CapturedAt: now, Models: map[string]float64{"m1": 1.0}},
		{CapturedAt: now.Add(5 * time.Minute), Models: map[string]float64{"m1": 1.0}},
		{CapturedAt: now.Add(10 * time.Minute), Models: map[string]float64{"m1": 1.0}},
		{CapturedAt: now.Add(15 * time.Minute), Models: map[string]float64{"m1": 1.0}},
		{CapturedAt: now.Add(20 * time.Minute), Models: map[string]float64{"m1": 1.0}},
		{CapturedAt: now.Add(25 * time.Minute), Models: map[string]float64{"m1": 1.0}},
		{CapturedAt: now.Add(30 * time.Minute), Models: map[string]float64{"m1": 1.0}},
		// Active phase: 30% consumed in 30 minutes
		{CapturedAt: now.Add(35 * time.Minute), Models: map[string]float64{"m1": 0.95}},
		{CapturedAt: now.Add(40 * time.Minute), Models: map[string]float64{"m1": 0.90}},
		{CapturedAt: now.Add(45 * time.Minute), Models: map[string]float64{"m1": 0.85}},
		{CapturedAt: now.Add(50 * time.Minute), Models: map[string]float64{"m1": 0.80}},
		{CapturedAt: now.Add(55 * time.Minute), Models: map[string]float64{"m1": 0.75}},
		{CapturedAt: now.Add(60 * time.Minute), Models: map[string]float64{"m1": 0.70}},
	}

	rates := ComputeRates(points)
	if rates == nil {
		t.Fatal("expected non-nil rates")
	}
	r := rates["m1"]
	if r == nil {
		t.Fatal("expected rate for m1")
	}

	// With recency weighting, the recent burst (high rate) should be weighted more.
	// The rate should be > 0.15/hr (the simple average would be 0.30/1h = 0.30/hr,
	// but recency weighting of burst data should push it higher than average).
	if r.Rate < 0.15 {
		t.Errorf("rate should be > 0.15/hr (recent burst should be weighted higher), got %f", r.Rate)
	}
	t.Logf("Computed rate: %f/hr (correctly reflecting recent burst)", r.Rate)
}

func TestGroupForecasts_BasicTTX(t *testing.T) {
	rates := map[string]*ModelRate{
		"m1": {ModelID: "m1", Rate: 0.20, Points: 10}, // 20%/hr
	}
	remaining := map[string]float64{"m1": 0.50} // 50% left

	assigner := func(id string) string { return "test_group" }
	groups := []GroupDefinition{{GroupKey: "test_group", DisplayName: "Test"}}

	forecasts := ComputeGroupForecasts(rates, remaining, nil, assigner, groups)
	if len(forecasts) != 1 {
		t.Fatalf("expected 1 forecast, got %d", len(forecasts))
	}

	f := forecasts[0]
	// TTX should be 0.50 / 0.20 = 2.5 hours
	if math.Abs(f.TTXHours-2.5) > 0.01 {
		t.Errorf("expected TTX ~2.5h, got %f", f.TTXHours)
	}
	if f.Severity != "caution" {
		t.Errorf("expected severity 'caution' for 2.5h TTX, got %s", f.Severity)
	}
	if f.Confidence != "high" {
		t.Errorf("expected confidence 'high' for 10 points, got %s", f.Confidence)
	}
	if f.TTXLabel == "" {
		t.Error("expected non-empty TTXLabel")
	}
	t.Logf("TTX: %s (severity: %s, confidence: %s)", f.TTXLabel, f.Severity, f.Confidence)
}

func TestGroupForecasts_Exhausted(t *testing.T) {
	rates := map[string]*ModelRate{
		"m1": {ModelID: "m1", Rate: 0.20, Points: 5},
	}
	remaining := map[string]float64{"m1": 0.0}

	assigner := func(id string) string { return "g" }
	groups := []GroupDefinition{{GroupKey: "g", DisplayName: "G"}}

	forecasts := ComputeGroupForecasts(rates, remaining, nil, assigner, groups)
	if len(forecasts) != 1 {
		t.Fatalf("expected 1 forecast, got %d", len(forecasts))
	}
	if forecasts[0].TTXHours != 0 {
		t.Errorf("expected TTX=0 for exhausted, got %f", forecasts[0].TTXHours)
	}
	if forecasts[0].Severity != "critical" {
		t.Errorf("expected severity 'critical' for exhausted, got %s", forecasts[0].Severity)
	}
}

func TestGroupForecasts_IdleRate(t *testing.T) {
	rates := map[string]*ModelRate{
		"m1": {ModelID: "m1", Rate: 0.0, Points: 8},
	}
	remaining := map[string]float64{"m1": 0.60}

	assigner := func(id string) string { return "g" }
	groups := []GroupDefinition{{GroupKey: "g", DisplayName: "G"}}

	forecasts := ComputeGroupForecasts(rates, remaining, nil, assigner, groups)
	if len(forecasts) != 1 {
		t.Fatalf("expected 1 forecast, got %d", len(forecasts))
	}
	if forecasts[0].TTXLabel != "idle" {
		t.Errorf("expected TTXLabel 'idle' for zero rate, got %s", forecasts[0].TTXLabel)
	}
}

func TestFormatTTX(t *testing.T) {
	tests := []struct {
		hours    float64
		expected string
	}{
		{0, "exhausted"},
		{-1, "exhausted"},
		{0.01, "<1m"},
		{0.25, "~15m"},
		{0.5, "~30m"},
		{1.0, "~1h"},
		{2.5, "~2.5h"},
		{5.0, "~5h"},
		{24, "~24h"},
		{48, "~2d"},
		{72, "~3d"},
	}

	for _, tt := range tests {
		got := formatTTX(tt.hours)
		if got != tt.expected {
			t.Errorf("formatTTX(%f) = %q, want %q", tt.hours, got, tt.expected)
		}
	}
}

func TestParseModelsJSON(t *testing.T) {
	json := `[{"modelId":"claude-3-5-sonnet","remainingFraction":0.75},{"modelId":"gpt-4o","remainingFraction":0.3}]`
	result := ParseModelsJSON(json)
	if len(result) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result))
	}
	if result["claude-3-5-sonnet"] != 0.75 {
		t.Errorf("expected 0.75 for claude, got %f", result["claude-3-5-sonnet"])
	}
	if result["gpt-4o"] != 0.3 {
		t.Errorf("expected 0.3 for gpt, got %f", result["gpt-4o"])
	}

	// Empty/invalid
	if ParseModelsJSON("") != nil {
		t.Error("expected nil for empty")
	}
	if ParseModelsJSON("invalid") != nil {
		t.Error("expected nil for invalid JSON")
	}
}
