// Package claude provides zero-dependency integration with Claude Code.
// It includes the statusline bridge for rate limit monitoring and deep session
// file parsing for token usage analytics.
package claude

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Defaults for bridge health checking.
const (
	DefaultStaleness    = 5 * time.Minute  // max age before data is considered stale
	healthCheckInterval = 30 * time.Minute // how often to verify bridge is still configured
)

const statuslineFileName = "claude-statusline.json"

// bridgeSnippet is a minimal inline bash snippet prepended to the user's
// statusline command. It saves stdin to a file, then pipes stdin through
// to the original command unchanged.
//
// How it works:
//  1. Reads all of stdin into $I
//  2. Saves $I to ~/.niyantra/data/claude-statusline.json (atomic via temp+mv)
//  3. Pipes $I to stdout (so the next command in the pipe gets it)
const bridgeSnippet = `bash -c 'I=$(cat);D=$HOME/.niyantra/data;mkdir -p "$D" 2>/dev/null;T="$D/.sl-$$";printf "%s" "$I">"$T"&&mv -f "$T" "$D/claude-statusline.json" 2>/dev/null||rm -f "$T" 2>/dev/null;printf "%s" "$I"'`

// bridgeMarker is a substring used to detect if the bridge snippet is already present.
const bridgeMarker = "claude-statusline.json"

// Window represents a single rate limit window from the statusline.
type Window struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"` // Unix epoch seconds
}

// RateLimits is the rate_limits portion of the Claude Code statusline JSON.
type RateLimits struct {
	FiveHour *Window `json:"five_hour,omitempty"`
	SevenDay *Window `json:"seven_day,omitempty"`
}

// statuslinePayload is the subset of the Claude Code statusline JSON we parse.
type statuslinePayload struct {
	RL RateLimits `json:"rate_limits"`
}

// DataDir returns the Niyantra data directory path.
func DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".niyantra", "data")
}

// DataFilePath returns the path to the statusline data file.
func DataFilePath() string {
	d := DataDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, statuslineFileName)
}

// ReadData reads and parses the statusline JSON file.
// Returns nil, nil if the file doesn't exist.
func ReadData() (*RateLimits, error) {
	path := DataFilePath()
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read statusline file: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var payload statuslinePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse statusline JSON: %w", err)
	}

	return &payload.RL, nil
}

// IsFresh returns true if the statusline file exists and was modified
// within the given maximum age.
func IsFresh(maxAge time.Duration) bool {
	path := DataFilePath()
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < maxAge
}

// isValidWindow checks if a rate limit window has plausible values.
func isValidWindow(w *Window) bool {
	if w == nil {
		return false
	}
	if w.UsedPercentage < 0 || w.UsedPercentage > 100 {
		return false
	}
	// resets_at must be a plausible Unix timestamp (after 2024, before 2030)
	if w.ResetsAt != 0 {
		if w.ResetsAt < 1704067200 || w.ResetsAt > 1893456000 {
			return false
		}
	}
	return true
}

// IsValid checks if the rate limit data is plausible.
// Returns false if the data looks corrupted, triggering API fallback.
func IsValid(rl *RateLimits) bool {
	if rl == nil {
		return false
	}
	hasValid := false
	if rl.FiveHour != nil {
		if !isValidWindow(rl.FiveHour) {
			return false
		}
		hasValid = true
	}
	if rl.SevenDay != nil {
		if !isValidWindow(rl.SevenDay) {
			return false
		}
		hasValid = true
	}
	return hasValid
}

// IsClaudeCodeInstalled checks if Claude Code appears to be installed
// by looking for the ~/.claude/ directory.
func IsClaudeCodeInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".claude"))
	return err == nil && info.IsDir()
}

// isBashAvailable checks whether bash is accessible in the system PATH,
// including WSL and Git Bash on Windows.
func isBashAvailable() bool {
	for _, name := range []string{"bash", "wsl"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

// claudeSettingsPath returns the path to Claude Code's user settings.json.
func claudeSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// readClaudeSettings reads and parses ~/.claude/settings.json.
func readClaudeSettings() (map[string]interface{}, error) {
	path := claudeSettingsPath()
	if path == "" {
		return nil, fmt.Errorf("cannot determine settings path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if len(data) == 0 {
		return make(map[string]interface{}), nil
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	return settings, nil
}

// writeClaudeSettings writes settings to ~/.claude/settings.json atomically.
func writeClaudeSettings(settings map[string]interface{}) error {
	path := claudeSettingsPath()
	if path == "" {
		return fmt.Errorf("cannot determine settings path")
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".niyantra-settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp settings file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp settings: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp settings: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename settings: %w", err)
	}

	return nil
}

// getCurrentStatusLineCommand extracts the current statusLine command from settings.
func getCurrentStatusLineCommand(settings map[string]interface{}) string {
	sl, ok := settings["statusLine"]
	if !ok || sl == nil {
		return ""
	}
	slMap, ok := sl.(map[string]interface{})
	if !ok {
		return ""
	}
	cmd, _ := slMap["command"].(string)
	return cmd
}

// setStatusLineCommand sets the statusLine command in settings.
func setStatusLineCommand(settings map[string]interface{}, command string) {
	settings["statusLine"] = map[string]interface{}{
		"type":    "command",
		"command": command,
	}
}

// hasBridgeSnippet returns true if the command already contains our bridge.
func hasBridgeSnippet(command string) bool {
	return strings.Contains(command, bridgeMarker)
}

// addBridgeSnippet prepends the save snippet to the user's command via a pipe.
func addBridgeSnippet(userCommand string) string {
	if userCommand == "" {
		return bridgeSnippet + " > /dev/null"
	}
	return bridgeSnippet + " | " + userCommand
}

// removeBridgeSnippet strips our snippet from the command.
func removeBridgeSnippet(command string) string {
	if idx := strings.Index(command, bridgeSnippet+" | "); idx == 0 {
		return strings.TrimSpace(command[len(bridgeSnippet+" | "):])
	}
	if command == bridgeSnippet+" > /dev/null" {
		return ""
	}
	return command
}

// SetupBridge configures the Claude Code statusline bridge.
// Idempotent — safe to call multiple times.
func SetupBridge(logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	if !IsClaudeCodeInstalled() {
		logger.Debug("Claude Code not detected, statusline bridge not configured")
		return nil
	}

	if runtime.GOOS == "windows" && !isBashAvailable() {
		logger.Info("Claude Code bridge requires bash (WSL/Git Bash) — skipped on this system")
		return nil
	}

	// Ensure data directory exists
	if d := DataDir(); d != "" {
		_ = os.MkdirAll(d, 0o700)
	}

	settings, err := readClaudeSettings()
	if err != nil {
		logger.Warn("Cannot read Claude Code settings, skipping bridge setup", "error", err)
		return nil
	}

	currentCmd := getCurrentStatusLineCommand(settings)
	if hasBridgeSnippet(currentCmd) {
		logger.Debug("Statusline bridge already configured")
		return nil
	}

	newCmd := addBridgeSnippet(currentCmd)
	setStatusLineCommand(settings, newCmd)
	if err := writeClaudeSettings(settings); err != nil {
		logger.Warn("Failed to configure statusline bridge", "error", err)
		return nil
	}

	if currentCmd == "" {
		logger.Info("Configured Claude Code statusline bridge (standalone)")
	} else {
		logger.Info("Configured Claude Code statusline bridge (prepended to existing command)")
	}
	return nil
}

// EnsureBridge performs a periodic health check to verify the bridge
// snippet is still present. S11: If missing, only logs a warning instead
// of re-injecting — respects user intent if they manually removed it.
var lastBridgeCheck time.Time

func EnsureBridge(logger *slog.Logger) {
	if time.Since(lastBridgeCheck) < healthCheckInterval {
		return
	}
	lastBridgeCheck = time.Now()

	if !IsClaudeCodeInstalled() {
		return
	}
	if runtime.GOOS == "windows" && !isBashAvailable() {
		return
	}

	settings, err := readClaudeSettings()
	if err != nil {
		return
	}

	currentCmd := getCurrentStatusLineCommand(settings)
	if hasBridgeSnippet(currentCmd) {
		return // Still healthy
	}

	// S11: Log a warning instead of aggressively re-injecting
	if logger != nil {
		logger.Warn("Claude Code statusline bridge snippet not found in settings — " +
			"it may have been manually removed. Re-enable in Settings to re-configure.")
	}
}

// DisableBridge removes the bridge snippet from Claude Code settings.
func DisableBridge(logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	settings, err := readClaudeSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	currentCmd := getCurrentStatusLineCommand(settings)
	if !hasBridgeSnippet(currentCmd) {
		logger.Info("Statusline bridge not configured, nothing to disable")
		return nil
	}

	originalCmd := removeBridgeSnippet(currentCmd)
	if originalCmd == "" {
		delete(settings, "statusLine")
	} else {
		setStatusLineCommand(settings, originalCmd)
	}
	if err := writeClaudeSettings(settings); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	// Clean up statusline data file
	if d := DataDir(); d != "" {
		_ = os.Remove(filepath.Join(d, statuslineFileName))
	}

	logger.Info("Claude Code statusline bridge disabled and cleaned up")
	return nil
}
