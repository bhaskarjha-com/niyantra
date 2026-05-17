package forecast

import (
	"math"
	"testing"
)

func TestMeanAndStdDev(t *testing.T) {
	tests := []struct {
		name     string
		data     []float64
		wantMean float64
		wantStd  float64
	}{
		{"empty", nil, 0, 0},
		{"single", []float64{5}, 5, 0},
		{"identical", []float64{3, 3, 3, 3}, 3, 0},
		{"basic", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 5, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mean, std := MeanAndStdDev(tt.data)
			if math.Abs(mean-tt.wantMean) > 0.01 {
				t.Errorf("mean = %v, want %v", mean, tt.wantMean)
			}
			if math.Abs(std-tt.wantStd) > 0.1 {
				t.Errorf("stddev = %v, want %v", std, tt.wantStd)
			}
		})
	}
}

func TestNoData(t *testing.T) {
	result := DetectAnomalies(nil, 100, DefaultConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 anomalies, got %d", len(result))
	}
}

func TestInsufficientHistory(t *testing.T) {
	data := map[string][]float64{
		"claude": {5, 5, 5}, // only 3 days, need 7
	}
	result := DetectAnomalies(data, 100, DefaultConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 anomalies for insufficient data, got %d", len(result))
	}
}

func TestNormalSpend(t *testing.T) {
	// Flat $5/day for 30 days — no anomaly
	data := map[string][]float64{
		"antigravity": make([]float64, 30),
	}
	for i := range data["antigravity"] {
		data["antigravity"][i] = 5.0
	}
	result := DetectAnomalies(data, 200, DefaultConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 anomalies for flat spend, got %d", len(result))
	}
}

func TestSpike2x(t *testing.T) {
	// $5/day avg, today $15 — should be a warning
	data := map[string][]float64{
		"claude": make([]float64, 30),
	}
	for i := 0; i < 29; i++ {
		data["claude"][i] = 5.0
	}
	data["claude"][29] = 15.0 // 3x spike
	result := DetectAnomalies(data, 200, DefaultConfig())
	if len(result) == 0 {
		t.Fatal("expected anomaly for 3x spike")
	}
	a := result[0]
	if a.Provider != "claude" {
		t.Errorf("provider = %v, want claude", a.Provider)
	}
	if a.ZScore < 2.0 {
		t.Errorf("z-score = %v, expected >= 2.0", a.ZScore)
	}
}

func TestSpikeCritical(t *testing.T) {
	// $5/day avg, today $25 — should be critical
	data := map[string][]float64{
		"cursor": make([]float64, 30),
	}
	for i := 0; i < 29; i++ {
		data["cursor"][i] = 5.0
	}
	data["cursor"][29] = 25.0 // 5x spike
	result := DetectAnomalies(data, 100, DefaultConfig())
	if len(result) == 0 {
		t.Fatal("expected critical anomaly for 5x spike")
	}
	if result[0].Severity != "critical" {
		t.Errorf("severity = %v, want critical", result[0].Severity)
	}
}

func TestZeroStdDev(t *testing.T) {
	// All identical values — no anomaly possible
	data := map[string][]float64{
		"gemini": make([]float64, 10),
	}
	for i := range data["gemini"] {
		data["gemini"][i] = 7.0
	}
	result := DetectAnomalies(data, 100, DefaultConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 anomalies for zero stddev, got %d", len(result))
	}
}

func TestMultipleProviders(t *testing.T) {
	// 2 providers, only claude spiking
	data := map[string][]float64{
		"claude":  make([]float64, 30),
		"copilot": make([]float64, 30),
	}
	for i := 0; i < 29; i++ {
		data["claude"][i] = 5.0
		data["copilot"][i] = 3.0
	}
	data["claude"][29] = 20.0  // spike
	data["copilot"][29] = 3.0  // normal

	result := DetectAnomalies(data, 200, DefaultConfig())
	if len(result) != 1 {
		t.Fatalf("expected 1 anomaly, got %d", len(result))
	}
	if result[0].Provider != "claude" {
		t.Errorf("wrong provider: %v", result[0].Provider)
	}
}

func TestBudgetProjection(t *testing.T) {
	data := map[string][]float64{
		"claude": make([]float64, 30),
	}
	for i := 0; i < 29; i++ {
		data["claude"][i] = 3.0
	}
	data["claude"][29] = 15.0 // spike

	result := DetectAnomalies(data, 100, DefaultConfig())
	if len(result) == 0 {
		t.Fatal("expected anomaly")
	}
	// projected: $15 * 30 = $450, budget $100, excess = $350
	if result[0].ProjectedImpact <= 0 {
		t.Errorf("expected positive projected impact, got %v", result[0].ProjectedImpact)
	}
}
