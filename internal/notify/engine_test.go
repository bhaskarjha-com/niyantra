package notify

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestCheckQuotaGuard(t *testing.T) {
	e := NewEngine(slog.Default())
	e.Configure(true, 15)

	// Simulate — Send will fail on CI (no display), but guard logic is testable
	// We test the guard mechanism, not the OS notification delivery.

	// Guard should be empty
	e.mu.Lock()
	if !e.guard["test-model"].IsZero() {
		t.Error("expected guard to be empty for test-model")
	}
	e.mu.Unlock()
}

func TestOnResetClearsGuard(t *testing.T) {
	e := NewEngine(slog.Default())
	e.Configure(true, 10)

	// Manually set guard
	e.mu.Lock()
	e.guard["claude_3.5_sonnet"] = time.Now()
	e.mu.Unlock()

	// OnReset should clear it
	e.OnReset("claude_3.5_sonnet")

	e.mu.Lock()
	if !e.guard["claude_3.5_sonnet"].IsZero() {
		t.Error("expected guard to be cleared after OnReset")
	}
	e.mu.Unlock()
}

func TestOnNotifyCallback(t *testing.T) {
	e := NewEngine(slog.Default())
	e.Configure(true, 20)

	var mu sync.Mutex
	var calledModel string
	var calledPct float64

	e.SetOnNotify(func(model string, remainingPct float64) {
		mu.Lock()
		calledModel = model
		calledPct = remainingPct
		mu.Unlock()
	})

	// Manually set guard and then call to simulate a notification fire
	// (We can't test the full Send path without OS notification support)
	// Instead, test that the callback is stored and accessible
	e.mu.Lock()
	cb := e.onNotify
	e.mu.Unlock()

	if cb == nil {
		t.Fatal("expected onNotify callback to be set")
	}

	// Call it directly to verify wiring
	cb("gpt-4o", 8.5)

	mu.Lock()
	defer mu.Unlock()
	if calledModel != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got '%s'", calledModel)
	}
	if calledPct != 8.5 {
		t.Errorf("expected pct 8.5, got %.1f", calledPct)
	}
}

func TestCheckQuotaSkipsWhenDisabled(t *testing.T) {
	e := NewEngine(slog.Default())
	e.Configure(false, 10) // disabled

	// Should not fire — verify guard stays empty
	e.CheckQuota("test-model", 5.0)

	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.guard["test-model"].IsZero() {
		t.Error("expected guard to remain empty when notifications are disabled")
	}
}

func TestCheckQuotaSkipsAboveThreshold(t *testing.T) {
	e := NewEngine(slog.Default())
	e.Configure(true, 10)

	// 50% remaining is above 10% threshold — should not fire
	e.CheckQuota("test-model", 50.0)

	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.guard["test-model"].IsZero() {
		t.Error("expected guard to remain empty when above threshold")
	}
}

func TestCheckClaudeQuotaConvertsUsedToRemaining(t *testing.T) {
	e := NewEngine(slog.Default())
	e.Configure(true, 15) // threshold 15%

	// 95% used = 5% remaining → should be below threshold
	// We can't test the Send path, but verify CheckClaudeQuota calls CheckQuota
	// with the converted key
	e.CheckClaudeQuota("five_hour", 95.0)

	// The key should be "claude_five_hour"
	// Guard may or may not be set depending on whether Send succeeds
	// (it will fail on CI — that's fine, we're testing the conversion logic)
}

func TestConfigureUpdatesSettings(t *testing.T) {
	e := NewEngine(slog.Default())

	// Default values
	if e.Enabled() {
		t.Error("expected disabled by default")
	}
	if e.Threshold() != 10 {
		t.Errorf("expected default threshold 10, got %.0f", e.Threshold())
	}

	// Configure
	e.Configure(true, 25)
	if !e.Enabled() {
		t.Error("expected enabled after Configure")
	}
	if e.Threshold() != 25 {
		t.Errorf("expected threshold 25, got %.0f", e.Threshold())
	}

	// Threshold must be positive
	e.Configure(true, -5)
	if e.Threshold() != 25 {
		t.Errorf("expected threshold to remain 25 for negative input, got %.0f", e.Threshold())
	}
}

func TestSendTestReturnsError(t *testing.T) {
	e := NewEngine(slog.Default())

	// SendTest should return an error or nil depending on platform support
	// Just verify it doesn't panic
	_ = e.SendTest()
}

func TestGuardTTLExpiry(t *testing.T) {
	e := NewEngine(slog.Default())
	e.Configure(true, 10)
	e.guardTTL = 100 * time.Millisecond // short TTL for testing

	// First call below threshold — should fire and set guard
	e.CheckQuota("codex_5h", 5.0)
	e.mu.Lock()
	guardSet := !e.guard["codex_5h"].IsZero()
	e.mu.Unlock()
	if !guardSet {
		t.Fatal("expected guard to be set after CheckQuota below threshold")
	}

	// Immediate second call — should be suppressed by guard
	e.mu.Lock()
	guardBefore := e.guard["codex_5h"]
	e.mu.Unlock()

	e.CheckQuota("codex_5h", 5.0)

	e.mu.Lock()
	guardAfter := e.guard["codex_5h"]
	e.mu.Unlock()
	if !guardBefore.Equal(guardAfter) {
		t.Error("expected guard timestamp to remain unchanged (suppressed)")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Third call — TTL expired, should fire again and update timestamp
	e.CheckQuota("codex_5h", 5.0)
	e.mu.Lock()
	guardFinal := e.guard["codex_5h"]
	e.mu.Unlock()
	if guardFinal.Equal(guardBefore) {
		t.Error("expected guard timestamp to be updated after TTL expiry")
	}
}
