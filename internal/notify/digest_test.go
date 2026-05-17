package notify

import (
	"sync"
	"testing"
	"time"
)

func TestDigestDisabled(t *testing.T) {
	var mu sync.Mutex
	var calls int
	b := NewDigestBatcher(5*time.Minute, func(title, body string) {
		mu.Lock()
		calls++
		mu.Unlock()
	})
	// Default is disabled
	added := b.Add(DigestAlert{Model: "gpt-4o", RemainingPct: 5})
	if added {
		t.Error("expected Add to return false when disabled")
	}
}

func TestDigestSingleAlert(t *testing.T) {
	var mu sync.Mutex
	var gotTitle, gotBody string
	done := make(chan struct{})

	b := NewDigestBatcher(50*time.Millisecond, func(title, body string) {
		mu.Lock()
		gotTitle = title
		gotBody = body
		mu.Unlock()
		close(done)
	})
	b.SetEnabled(true)

	added := b.Add(DigestAlert{Model: "gpt-4o", RemainingPct: 5, Timestamp: time.Now()})
	if !added {
		t.Fatal("expected Add to return true when enabled")
	}

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("digest did not flush within timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotTitle == "" {
		t.Error("expected non-empty title")
	}
	if gotBody == "" {
		t.Error("expected non-empty body")
	}
}

func TestDigestBatching(t *testing.T) {
	var mu sync.Mutex
	var flushCount int
	var lastBody string
	done := make(chan struct{}, 1)

	b := NewDigestBatcher(100*time.Millisecond, func(title, body string) {
		mu.Lock()
		flushCount++
		lastBody = body
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})
	b.SetEnabled(true)

	// Add 3 alerts rapidly (within the window)
	b.Add(DigestAlert{Model: "gpt-4o", RemainingPct: 5})
	b.Add(DigestAlert{Model: "claude-3", RemainingPct: 8})
	b.Add(DigestAlert{Model: "gemini-pro", RemainingPct: 3})

	if b.PendingCount() != 3 {
		t.Errorf("expected 3 pending, got %d", b.PendingCount())
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("digest did not flush")
	}

	mu.Lock()
	defer mu.Unlock()
	if flushCount != 1 {
		t.Errorf("expected 1 flush, got %d", flushCount)
	}
	// Body should mention all 3 models
	if lastBody == "" {
		t.Error("expected non-empty body")
	}
}

func TestDigestEarlyFlush(t *testing.T) {
	var mu sync.Mutex
	var flushCount int
	done := make(chan struct{}, 1)

	b := NewDigestBatcher(10*time.Second, func(title, body string) {
		mu.Lock()
		flushCount++
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})
	b.SetEnabled(true)

	// Add 5 alerts (maxBatch default) — should flush immediately
	for i := 0; i < 5; i++ {
		b.Add(DigestAlert{Model: "model-" + string(rune('a'+i)), RemainingPct: float64(i + 1)})
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("early flush did not trigger")
	}

	mu.Lock()
	defer mu.Unlock()
	if flushCount != 1 {
		t.Errorf("expected 1 flush from early trigger, got %d", flushCount)
	}
	if b.PendingCount() != 0 {
		t.Errorf("expected 0 pending after flush, got %d", b.PendingCount())
	}
}

func TestDigestFormatSingle(t *testing.T) {
	title, body := formatDigest([]DigestAlert{
		{Model: "gpt-4o", RemainingPct: 5},
	})
	if title == "" || body == "" {
		t.Error("expected non-empty title and body for single alert")
	}
}

func TestDigestFormatMultiple(t *testing.T) {
	title, body := formatDigest([]DigestAlert{
		{Model: "gpt-4o", RemainingPct: 5},
		{Model: "claude-3", RemainingPct: 8},
		{Model: "gemini-pro", RemainingPct: 3},
	})
	if title == "" || body == "" {
		t.Error("expected non-empty title and body")
	}
	// Title should mention count
	if title != "⚠️ 3 quota alerts" {
		t.Errorf("unexpected title: %s", title)
	}
}
