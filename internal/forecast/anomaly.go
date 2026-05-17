// Package forecast provides cost anomaly detection using statistical analysis.
// Uses rolling Z-score algorithm to detect abnormal spend spikes.
// Stateless — pure functions with no side effects.
package forecast

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Anomaly represents a detected cost spike for a provider.
type Anomaly struct {
	Provider        string    `json:"provider"`
	DetectedAt      time.Time `json:"detectedAt"`
	CurrentValue    float64   `json:"currentValue"`    // today's spend
	Mean30d         float64   `json:"mean30d"`          // 30-day average
	StdDev30d       float64   `json:"stdDev30d"`        // 30-day standard deviation
	ZScore          float64   `json:"zScore"`            // how many σ above mean
	Multiplier      float64   `json:"multiplier"`        // "3.2x above average"
	Severity        string    `json:"severity"`          // "warning" (2-3σ) or "critical" (>3σ)
	ProjectedImpact float64   `json:"projectedImpact"`   // estimated monthly excess
	Message         string    `json:"message"`           // human-readable summary
}

// AnomalyConfig controls detection sensitivity.
type AnomalyConfig struct {
	Threshold  float64 // Z-score threshold (default: 2.0)
	MinDays    int     // minimum history required (default: 7)
	WindowDays int     // rolling window size (default: 30)
}

// DefaultConfig returns the recommended anomaly detection configuration.
func DefaultConfig() AnomalyConfig {
	return AnomalyConfig{
		Threshold:  2.0,
		MinDays:    7,
		WindowDays: 30,
	}
}

// DetectAnomalies analyzes recent spend data for each provider and returns
// any detected cost spikes. Uses a rolling Z-score algorithm.
//
// dailySpendByProvider: map["antigravity"] → []float64 (daily spend, newest last)
// budget: monthly budget (0 = no budget set)
func DetectAnomalies(
	dailySpendByProvider map[string][]float64,
	budget float64,
	cfg AnomalyConfig,
) []Anomaly {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 2.0
	}
	if cfg.MinDays <= 0 {
		cfg.MinDays = 7
	}
	if cfg.WindowDays <= 0 {
		cfg.WindowDays = 30
	}

	var anomalies []Anomaly

	for provider, dailySpend := range dailySpendByProvider {
		if len(dailySpend) < cfg.MinDays {
			continue // insufficient history
		}

		// Take last WindowDays (or all if fewer)
		window := dailySpend
		if len(window) > cfg.WindowDays {
			window = window[len(window)-cfg.WindowDays:]
		}

		// Compute mean and stddev
		mean, stddev := MeanAndStdDev(window)
		if stddev == 0 {
			continue // no variation = no anomalies
		}

		// Today is the last element
		today := window[len(window)-1]
		z := (today - mean) / stddev

		if z > cfg.Threshold {
			multiplier := today / mean
			if mean == 0 {
				multiplier = 0
			}
			severity := "warning"
			if z > 3.0 {
				severity = "critical"
			}

			// Project monthly impact
			projected := 0.0
			if budget > 0 {
				projectedMonthly := today * 30.0
				if projectedMonthly > budget {
					projected = math.Round((projectedMonthly-budget)*100) / 100
				}
			}

			providerTitle := strings.ToUpper(provider[:1]) + provider[1:]
			anomalies = append(anomalies, Anomaly{
				Provider:        provider,
				DetectedAt:      time.Now().UTC(),
				CurrentValue:    math.Round(today*100) / 100,
				Mean30d:         math.Round(mean*100) / 100,
				StdDev30d:       math.Round(stddev*100) / 100,
				ZScore:          math.Round(z*10) / 10,
				Multiplier:      math.Round(multiplier*10) / 10,
				Severity:        severity,
				ProjectedImpact: projected,
				Message: fmt.Sprintf(
					"%s spend is %.1fx above 30-day average ($%.2f vs $%.2f avg)",
					providerTitle, multiplier, today, mean,
				),
			})
		}
	}

	// Sort by severity (critical first), then by Z-score descending
	sort.Slice(anomalies, func(i, j int) bool {
		if anomalies[i].Severity != anomalies[j].Severity {
			return anomalies[i].Severity == "critical"
		}
		return anomalies[i].ZScore > anomalies[j].ZScore
	})

	return anomalies
}

// MeanAndStdDev computes the arithmetic mean and population standard deviation.
func MeanAndStdDev(data []float64) (float64, float64) {
	n := float64(len(data))
	if n == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	mean := sum / n

	variance := 0.0
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	variance /= n
	return mean, math.Sqrt(variance)
}
