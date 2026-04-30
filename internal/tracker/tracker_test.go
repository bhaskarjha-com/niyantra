package tracker

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// openTestStore creates an in-memory store for tracker tests.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestProcessMultiAccountNoContamination verifies that processing Account A
// does not affect reset detection or delta calculations for Account B.
// This is the C6 regression test.
func TestProcessMultiAccountNoContamination(t *testing.T) {
	s := openTestStore(t)
	tr := New(s, slog.Default())

	now := time.Now().UTC()
	resetTime := now.Add(5 * time.Hour)

	// Create two accounts
	accountA, _ := s.GetOrCreateAccount("a@test.com", "Pro")
	accountB, _ := s.GetOrCreateAccount("b@test.com", "Free")

	// Process Account A with modelX at 0.8 remaining
	snapA := &client.Snapshot{
		CapturedAt: now,
		Models: []client.ModelQuota{
			{ModelID: "modelX", Label: "Model X", RemainingFraction: 0.8, ResetTime: &resetTime},
		},
	}
	if err := tr.Process(snapA, accountA); err != nil {
		t.Fatalf("Process Account A: %v", err)
	}

	// Process Account B with modelX at 0.4 remaining
	snapB := &client.Snapshot{
		CapturedAt: now.Add(1 * time.Second),
		Models: []client.ModelQuota{
			{ModelID: "modelX", Label: "Model X", RemainingFraction: 0.4, ResetTime: &resetTime},
		},
	}
	if err := tr.Process(snapB, accountB); err != nil {
		t.Fatalf("Process Account B: %v", err)
	}

	// Now process Account B again with 0.3 — delta should be 0.1 (0.4 → 0.3),
	// NOT 0.5 (if it was compared against Account A's 0.8)
	snapB2 := &client.Snapshot{
		CapturedAt: now.Add(2 * time.Second),
		Models: []client.ModelQuota{
			{ModelID: "modelX", Label: "Model X", RemainingFraction: 0.3, ResetTime: &resetTime},
		},
	}
	if err := tr.Process(snapB2, accountB); err != nil {
		t.Fatalf("Process Account B (2nd): %v", err)
	}

	// Verify Account B's cycle has correct delta (~0.1, not ~0.5)
	cycle, err := s.ActiveCycle("modelX", accountB)
	if err != nil || cycle == nil {
		t.Fatalf("ActiveCycle for B: %v", err)
	}

	// The delta should be around 0.1 (from 0.4 to 0.3)
	if cycle.TotalDelta > 0.15 {
		t.Errorf("Account B total delta = %f, expected ~0.1 (cross-account contamination detected)", cycle.TotalDelta)
	}

	// Verify Account A's cycle is unaffected
	cycleA, err := s.ActiveCycle("modelX", accountA)
	if err != nil || cycleA == nil {
		t.Fatalf("ActiveCycle for A: %v", err)
	}
	if cycleA.TotalDelta != 0 {
		t.Errorf("Account A total delta = %f, expected 0 (no usage change)", cycleA.TotalDelta)
	}
}

// TestProcessConcurrentSafe verifies that concurrent Process() calls
// don't cause a map panic. This is the C7 regression test.
func TestProcessConcurrentSafe(t *testing.T) {
	s := openTestStore(t)
	tr := New(s, slog.Default())

	now := time.Now().UTC()
	resetTime := now.Add(5 * time.Hour)

	accountA, _ := s.GetOrCreateAccount("a@test.com", "Pro")
	accountB, _ := s.GetOrCreateAccount("b@test.com", "Free")

	var wg sync.WaitGroup
	wg.Add(2)

	// Launch 2 goroutines both calling Process()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			snap := &client.Snapshot{
				CapturedAt: now.Add(time.Duration(i) * time.Second),
				Models: []client.ModelQuota{
					{ModelID: "model1", Label: "M1", RemainingFraction: 0.8 - float64(i)*0.01, ResetTime: &resetTime},
				},
			}
			tr.Process(snap, accountA)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			snap := &client.Snapshot{
				CapturedAt: now.Add(time.Duration(i) * time.Second),
				Models: []client.ModelQuota{
					{ModelID: "model1", Label: "M1", RemainingFraction: 0.6 - float64(i)*0.01, ResetTime: &resetTime},
				},
			}
			tr.Process(snap, accountB)
		}
	}()

	// If there's no mutex, this panics with "concurrent map read and map write"
	wg.Wait()
}
