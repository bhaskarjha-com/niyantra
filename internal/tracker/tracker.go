package tracker

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// Tracker manages reset cycle detection and usage calculation for Antigravity models.
// Uses 3-method reset detection: time shift, fraction increase, and time-based.
type Tracker struct {
	mu             sync.RWMutex
	store          *store.Store
	logger         *slog.Logger
	lastFractions  map[string]float64   // accountID:model_id → last remaining fraction
	lastResetTimes map[string]time.Time // accountID:model_id → last reported reset time
	hasBaseline    bool

	onReset func(modelID string) // callback when a model reset is detected
}

// New creates a new Tracker.
func New(store *store.Store, logger *slog.Logger) *Tracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Tracker{
		store:          store,
		logger:         logger,
		lastFractions:  make(map[string]float64),
		lastResetTimes: make(map[string]time.Time),
	}
}

// LoadBaseline seeds the tracker's in-memory state from the latest snapshots
// per account in the database. This prevents orphaned cycles after restart (N5).
func (t *Tracker) LoadBaseline() {
	t.mu.Lock()
	defer t.mu.Unlock()

	snapshots, err := t.store.LatestPerAccount()
	if err != nil {
		t.logger.Warn("Tracker: failed to load baseline from DB", "error", err)
		return
	}

	count := 0
	for _, snap := range snapshots {
		if snap == nil {
			continue
		}
		for _, m := range snap.Models {
			key := fmt.Sprintf("%d:%s", snap.AccountID, m.ModelID)
			t.lastFractions[key] = m.RemainingFraction
			if m.ResetTime != nil {
				t.lastResetTimes[key] = *m.ResetTime
			}
			count++
		}
	}

	if count > 0 {
		t.hasBaseline = true
		t.logger.Info("Tracker: loaded baseline from DB", "models", count, "accounts", len(snapshots))
	}
}

// SetOnReset registers a callback invoked when a model reset is detected.
func (t *Tracker) SetOnReset(fn func(string)) {
	t.onReset = fn
}

// Process iterates over all models in the snapshot, detects resets, and updates cycles.
func (t *Tracker) Process(snap *client.Snapshot, accountID int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if snap == nil || len(snap.Models) == 0 {
		return nil
	}

	for _, model := range snap.Models {
		if err := t.processModel(model, snap.CapturedAt, accountID); err != nil {
			return fmt.Errorf("tracker: %s: %w", model.ModelID, err)
		}
	}

	t.hasBaseline = true
	return nil
}

// processModel handles cycle detection and tracking for a single model.
func (t *Tracker) processModel(model client.ModelQuota, capturedAt time.Time, accountID int64) error {
	modelID := model.ModelID
	if modelID == "" {
		return nil
	}

	currentUsage := 1.0 - model.RemainingFraction

	// Query active cycle for this model
	cycle, err := t.store.ActiveCycle(modelID, accountID)
	if err != nil {
		return fmt.Errorf("query active cycle: %w", err)
	}

	if cycle == nil {
		// First snapshot for this model — create new cycle
		if _, err := t.store.CreateCycle(modelID, accountID, capturedAt, model.ResetTime); err != nil {
			return fmt.Errorf("create cycle: %w", err)
		}
		if err := t.store.UpdateCycle(modelID, accountID, currentUsage, 0, 1); err != nil {
			return fmt.Errorf("set initial peak: %w", err)
		}
		mapKey := fmt.Sprintf("%d:%s", accountID, modelID)
		t.lastFractions[mapKey] = model.RemainingFraction
		if model.ResetTime != nil {
			t.lastResetTimes[mapKey] = *model.ResetTime
		}
		t.logger.Debug("Created new cycle",
			"model", modelID, "label", model.Label,
			"resetTime", model.ResetTime, "initialUsage", currentUsage,
		)
		return nil
	}

	// ── Reset detection (3 methods) ────────────────────────────────

	resetDetected := false
	resetReason := ""

	// Method 1: Reset time changed significantly (>10 min shift)
	mapKey := fmt.Sprintf("%d:%s", accountID, modelID)

	if model.ResetTime != nil {
		if lastReset, ok := t.lastResetTimes[mapKey]; ok {
			diff := model.ResetTime.Sub(lastReset)
			if diff < 0 {
				diff = -diff
			}
			if diff > 10*time.Minute {
				resetDetected = true
				resetReason = "reset_time shifted"
			}
		}
	}

	// Method 2: Remaining fraction increased significantly (>10% = quota replenished)
	if !resetDetected && t.hasBaseline {
		if lastFraction, ok := t.lastFractions[mapKey]; ok {
			if model.RemainingFraction > lastFraction+0.1 {
				resetDetected = true
				resetReason = "remaining_fraction increased"
			}
		}
	}

	// Method 3: Time-based — reset time passed AND remaining increased
	if !resetDetected && cycle.ResetTime != nil && capturedAt.After(*cycle.ResetTime) {
		if lastFraction, ok := t.lastFractions[mapKey]; ok {
			if model.RemainingFraction > lastFraction {
				resetDetected = true
				resetReason = "time-based (reset passed + remaining increased)"
			}
		}
	}

	// ── Handle reset ──────────────────────────────────────────────

	if resetDetected {
		// Determine cycle end time
		cycleEndTime := capturedAt
		if cycle.ResetTime != nil && capturedAt.After(*cycle.ResetTime) {
			cycleEndTime = *cycle.ResetTime
		}

		// Close old cycle
		if err := t.store.CloseCycle(modelID, accountID, cycleEndTime, cycle.PeakUsage, cycle.TotalDelta); err != nil {
			return fmt.Errorf("close cycle: %w", err)
		}

		// Create new cycle
		if _, err := t.store.CreateCycle(modelID, accountID, capturedAt, model.ResetTime); err != nil {
			return fmt.Errorf("create new cycle: %w", err)
		}
		if err := t.store.UpdateCycle(modelID, accountID, currentUsage, 0, 1); err != nil {
			return fmt.Errorf("set initial peak: %w", err)
		}

		// Update tracking state
		t.lastFractions[mapKey] = model.RemainingFraction
		if model.ResetTime != nil {
			t.lastResetTimes[mapKey] = *model.ResetTime
		}

		t.logger.Info("Detected model reset",
			"model", modelID,
			"reason", resetReason,
			"oldResetTime", cycle.ResetTime,
			"newResetTime", model.ResetTime,
		)

		// Log reset event
		t.store.LogInfo("tracker", "model_reset", "", map[string]interface{}{
			"model": modelID, "reason": resetReason,
		})

		if t.onReset != nil {
			t.onReset(modelID)
		}
		return nil
	}

	// ── Same cycle — update delta + peak ──────────────────────────

	newSnapshotCount := cycle.SnapshotCount + 1

	if t.hasBaseline {
		if lastFraction, ok := t.lastFractions[mapKey]; ok {
			usageDelta := lastFraction - model.RemainingFraction
			if usageDelta > 0 {
				cycle.TotalDelta += usageDelta
			}
			if currentUsage > cycle.PeakUsage {
				cycle.PeakUsage = currentUsage
			}
		}
	} else {
		if currentUsage > cycle.PeakUsage {
			cycle.PeakUsage = currentUsage
		}
	}

	if err := t.store.UpdateCycle(modelID, accountID, cycle.PeakUsage, cycle.TotalDelta, newSnapshotCount); err != nil {
		return fmt.Errorf("update cycle: %w", err)
	}

	// Update tracking state
	t.lastFractions[mapKey] = model.RemainingFraction
	if model.ResetTime != nil {
		t.lastResetTimes[mapKey] = *model.ResetTime
	}

	return nil
}
