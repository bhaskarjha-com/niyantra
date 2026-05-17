package notify

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// DigestAlert is a single alert queued for batching.
type DigestAlert struct {
	Model        string
	RemainingPct float64
	Timestamp    time.Time
}

// DigestBatcher collects alerts and flushes them as a single digest.
// Thread-safe via mutex. Uses time.AfterFunc for delayed flush.
type DigestBatcher struct {
	mu         sync.Mutex
	enabled    bool
	windowDur  time.Duration   // batch window (default 5 min)
	maxBatch   int             // flush early if batch >= this (default 5)
	pending    []DigestAlert   // current batch
	flushTimer *time.Timer     // fires when window expires
	onFlush    func(title string, body string) // callback to deliver the digest
}

// NewDigestBatcher creates a batcher with the given window and flush callback.
func NewDigestBatcher(window time.Duration, onFlush func(title string, body string)) *DigestBatcher {
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &DigestBatcher{
		enabled:   false, // disabled by default
		windowDur: window,
		maxBatch:  5,
		onFlush:   onFlush,
	}
}

// SetEnabled toggles digest mode on/off.
func (d *DigestBatcher) SetEnabled(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = enabled
	if !enabled && d.flushTimer != nil {
		d.flushTimer.Stop()
		// Flush any pending immediately when disabled
		if len(d.pending) > 0 {
			pending := d.pending
			d.pending = nil
			d.flushTimer = nil
			title, body := formatDigest(pending)
			go d.onFlush(title, body)
		}
	}
}

// SetWindow updates the batch window duration.
func (d *DigestBatcher) SetWindow(dur time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if dur > 0 {
		d.windowDur = dur
	}
}

// IsEnabled returns whether digest mode is active.
func (d *DigestBatcher) IsEnabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enabled
}

// Add queues an alert into the current batch.
// Returns true if the alert was batched, false if digest is disabled
// (caller should deliver immediately).
func (d *DigestBatcher) Add(alert DigestAlert) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled {
		return false // not batched — caller delivers immediately
	}

	// Append to pending batch
	d.pending = append(d.pending, alert)

	// Start timer if this is the first alert in the batch
	if len(d.pending) == 1 {
		d.flushTimer = time.AfterFunc(d.windowDur, d.flush)
	}

	// Early flush if batch is large enough
	if len(d.pending) >= d.maxBatch {
		if d.flushTimer != nil {
			d.flushTimer.Stop()
		}
		pending := d.pending
		d.pending = nil
		d.flushTimer = nil
		title, body := formatDigest(pending)
		go d.onFlush(title, body)
	}

	return true
}

// flush is called by the timer goroutine.
func (d *DigestBatcher) flush() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pending) == 0 {
		return
	}
	pending := d.pending
	d.pending = nil
	d.flushTimer = nil
	title, body := formatDigest(pending)
	go d.onFlush(title, body)
}

// PendingCount returns the number of alerts in the current batch (for testing).
func (d *DigestBatcher) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

// formatDigest creates a human-readable summary from batched alerts.
func formatDigest(alerts []DigestAlert) (title string, body string) {
	if len(alerts) == 1 {
		a := alerts[0]
		title = fmt.Sprintf("⚠️ %s at %.0f%%", a.Model, a.RemainingPct)
		body = fmt.Sprintf("%.1f%% remaining — consider switching models", a.RemainingPct)
		return
	}

	// Multiple alerts → digest summary
	title = fmt.Sprintf("⚠️ %d quota alerts", len(alerts))

	var lines []string
	lowestPct := 100.0
	for _, a := range alerts {
		lines = append(lines, fmt.Sprintf("• %s: %.0f%% remaining", a.Model, a.RemainingPct))
		if a.RemainingPct < lowestPct {
			lowestPct = a.RemainingPct
		}
	}
	body = fmt.Sprintf("%d models below threshold (lowest: %.0f%%).\n%s",
		len(alerts), lowestPct, strings.Join(lines, "\n"))

	return
}
