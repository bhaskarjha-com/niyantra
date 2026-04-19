package agent

import (
	"context"
	"log/slog"
	"sync"
)

// Manager handles the lifecycle of the auto-capture polling agent.
type Manager struct {
	mu     sync.Mutex
	cancel context.CancelFunc // nil = not running
	agent  *PollingAgent
	logger *slog.Logger
}

// NewManager creates a new agent lifecycle manager.
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{logger: logger}
}

// Start launches the polling agent in a background goroutine.
// If already running, this is a no-op.
func (m *Manager) Start(agent *PollingAgent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		return // already running
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.agent = agent

	go func() {
		if err := agent.Run(ctx); err != nil && ctx.Err() == nil {
			m.logger.Error("Polling agent error", "error", err)
		}
		m.mu.Lock()
		m.cancel = nil
		m.agent = nil
		m.mu.Unlock()
	}()
}

// Stop cancels the running polling agent and waits briefly.
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.agent = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
		m.logger.Info("Auto-capture agent stopped")
	}
}

// IsRunning returns whether the polling agent is currently active.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancel != nil
}

// Agent returns the current polling agent (nil if not running).
func (m *Manager) Agent() *PollingAgent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.agent
}
