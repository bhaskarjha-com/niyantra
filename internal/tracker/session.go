package tracker

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// SessionManager provides usage-based session detection for any provider.
// A session starts when quota values change, and closes after an idle
// timeout with no further changes. Each provider gets its own SessionManager.
type SessionManager struct {
	store       *store.Store
	provider    string
	idleTimeout time.Duration
	logger      *slog.Logger

	sessionID        int64     // 0 = no active session
	lastActivityTime time.Time // last time usage changed
	prevValues       []float64 // previous poll values for comparison
	hasPrev          bool      // true after first poll (baseline established)
}

// NewSessionManager creates a SessionManager for the given provider.
func NewSessionManager(st *store.Store, provider string, idleTimeout time.Duration, logger *slog.Logger) *SessionManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &SessionManager{
		store:       st,
		provider:    provider,
		idleTimeout: idleTimeout,
		logger:      logger,
	}
}

// ReportPoll is called after each successful poll with current usage values.
// values is a flat slice of comparable numbers (remaining percentages, utilization, etc.).
// Returns true if usage changed (session is active).
func (sm *SessionManager) ReportPoll(values []float64) bool {
	now := time.Now().UTC()
	changed := sm.hasChanged(values)

	// Save old values before overwriting (needed for session start baseline)
	oldPrev := make([]float64, len(sm.prevValues))
	copy(oldPrev, sm.prevValues)

	// Update stored previous values
	sm.prevValues = make([]float64, len(values))
	copy(sm.prevValues, values)
	if !sm.hasPrev {
		sm.hasPrev = true
		return false // first poll is always baseline
	}

	if changed {
		if sm.sessionID == 0 {
			// Start new session with baseline values
			startJSON, _ := json.Marshal(oldPrev)
			id, err := sm.store.CreateSession(sm.provider, now, oldPrev)
			if err != nil {
				sm.logger.Error("Failed to create session", "provider", sm.provider, "error", err)
				return true
			}
			sm.sessionID = id
			sm.lastActivityTime = now
			sm.logger.Info("Usage session started",
				"provider", sm.provider,
				"session_id", id,
				"start_values", string(startJSON))
		}

		sm.lastActivityTime = now
		sm.updateSession(values)
		return true
	}

	// No usage change
	if sm.sessionID != 0 {
		if now.Sub(sm.lastActivityTime) > sm.idleTimeout {
			// Idle timeout exceeded → close session
			sm.closeSession(sm.lastActivityTime.Add(sm.idleTimeout))
		} else {
			// Still within idle window — count the snapshot
			sm.updateSession(values)
		}
	}

	return false
}

// Close closes any active session (called on agent shutdown).
func (sm *SessionManager) Close() {
	if sm.sessionID == 0 {
		return
	}
	sm.closeSession(time.Now().UTC())
}

// hasChanged compares current values with previous values.
func (sm *SessionManager) hasChanged(values []float64) bool {
	if !sm.hasPrev {
		return false
	}
	if len(values) != len(sm.prevValues) {
		return true
	}
	for i, v := range values {
		if v != sm.prevValues[i] {
			return true
		}
	}
	return false
}

// closeSession closes the current active session.
func (sm *SessionManager) closeSession(endTime time.Time) {
	if err := sm.store.CloseSession(sm.sessionID, endTime); err != nil {
		sm.logger.Error("Failed to close session",
			"provider", sm.provider, "session_id", sm.sessionID, "error", err)
	} else {
		sm.logger.Info("Usage session ended",
			"provider", sm.provider, "session_id", sm.sessionID)
	}
	sm.sessionID = 0
}

// updateSession increments snap count and updates peak values.
func (sm *SessionManager) updateSession(values []float64) {
	if err := sm.store.IncrementSessionSnapCount(sm.sessionID); err != nil {
		sm.logger.Error("Failed to increment snap count", "provider", sm.provider, "error", err)
	}
	if err := sm.store.UpdateSessionPeakValues(sm.sessionID, values); err != nil {
		sm.logger.Error("Failed to update peak values", "provider", sm.provider, "error", err)
	}
}
