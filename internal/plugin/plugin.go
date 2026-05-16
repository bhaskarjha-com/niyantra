// Package plugin provides an extensible plugin system for custom data sources.
//
// Plugins are external executables discovered from ~/.niyantra/plugins/*/plugin.json.
// On each poll cycle, Niyantra spawns the plugin subprocess, sends a JSON request
// via stdin, and reads a JSON response from stdout. This architecture is inspired
// by Telegraf's exec input plugin — see docs/adr/0001-plugin-system-architecture.md.
//
// Zero external dependencies: uses only os/exec + encoding/json from the stdlib.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Manifest represents a plugin.json manifest file.
type Manifest struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Author       string                 `json:"author"`
	EntryPoint   string                 `json:"entryPoint"`
	Timeout      int                    `json:"timeout"` // seconds, default 30
	Capabilities []string               `json:"capabilities"`
	Config       map[string]ConfigField `json:"config"`
}

// ConfigField describes a single configurable field in the plugin manifest.
type ConfigField struct {
	Type     string `json:"type"`              // string, int, bool
	Label    string `json:"label"`
	Default  string `json:"default,omitempty"`
	Required bool   `json:"required,omitempty"`
	Secret   bool   `json:"secret,omitempty"`
}

// Plugin represents a discovered, validated plugin ready for execution.
type Plugin struct {
	Manifest  Manifest          `json:"manifest"`
	Dir       string            `json:"dir"`       // absolute path to plugin dir
	EntryPath string            `json:"entryPath"` // absolute path to entry point executable
	Enabled   bool              `json:"enabled"`
	Config    map[string]string `json:"config"` // user-configured values from SQLite
}

// CaptureRequest is the JSON payload sent to a plugin's stdin.
type CaptureRequest struct {
	Action string            `json:"action"`
	Config map[string]string `json:"config,omitempty"`
}

// CaptureResult is the JSON response read from a plugin's stdout.
type CaptureResult struct {
	Status string      `json:"status"` // "ok" or "error"
	Error  string      `json:"error,omitempty"`
	Data   CaptureData `json:"data,omitempty"`
}

// CaptureData holds the captured usage metrics from a plugin.
type CaptureData struct {
	Provider     string            `json:"provider"`
	Label        string            `json:"label"`
	Email        string            `json:"email"`
	UsagePct     float64           `json:"usage_pct"`
	UsageDisplay string            `json:"usage_display"`
	Plan         string            `json:"plan"`
	Models       []PluginModel     `json:"models"`
	Metadata     map[string]any    `json:"metadata"`
}

// PluginModel represents a single model/resource tracked by a plugin.
type PluginModel struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	UsagePct float64 `json:"usage_pct"`
	Detail   string  `json:"detail"`
}

// Validate checks that a manifest has all required fields.
func (m *Manifest) Validate() error {
	if m.ID == "" {
		return errors.New("plugin manifest: id is required")
	}
	if m.Name == "" {
		return errors.New("plugin manifest: name is required")
	}
	if m.EntryPoint == "" {
		return errors.New("plugin manifest: entryPoint is required")
	}
	// Reject path traversal in entryPoint
	// Check filepath.IsAbs for OS-native check, plus explicit / and \ prefix for cross-platform safety
	if strings.Contains(m.EntryPoint, "..") || filepath.IsAbs(m.EntryPoint) ||
		strings.HasPrefix(m.EntryPoint, "/") || strings.HasPrefix(m.EntryPoint, "\\") {
		return fmt.Errorf("plugin manifest: entryPoint must be a relative path within the plugin directory, got %q", m.EntryPoint)
	}
	// Reject IDs with special characters (used as SQLite keys and URL path segments)
	for _, c := range m.ID {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("plugin manifest: id contains invalid character %q (allowed: a-z, A-Z, 0-9, -, _)", string(c))
		}
	}
	if m.Timeout < 0 {
		return errors.New("plugin manifest: timeout must be non-negative")
	}
	return nil
}

// EffectiveTimeout returns the timeout in seconds, defaulting to 30 if unset.
func (m *Manifest) EffectiveTimeout() int {
	if m.Timeout <= 0 {
		return 30
	}
	return m.Timeout
}

// DefaultPluginsDir returns the default plugins directory (~/.niyantra/plugins).
func DefaultPluginsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".niyantra", "plugins")
}

// Discover scans the plugins directory and returns all valid plugins.
// Invalid plugins (bad JSON, missing fields, missing entry point) are logged
// and skipped — they do not cause the entire discovery to fail.
func Discover(pluginsDir string) ([]*Plugin, []error) {
	var plugins []*Plugin
	var errs []error

	if pluginsDir == "" {
		pluginsDir = DefaultPluginsDir()
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no plugins dir is fine
		}
		return nil, []error{fmt.Errorf("plugin: reading plugins dir %s: %w", pluginsDir, err)}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(pluginsDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.json")

		p, err := loadPlugin(pluginDir, manifestPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("plugin %s: %w", entry.Name(), err))
			continue
		}

		plugins = append(plugins, p)
	}

	return plugins, errs
}

// loadPlugin reads and validates a single plugin from its directory.
func loadPlugin(pluginDir, manifestPath string) (*Plugin, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading plugin.json: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing plugin.json: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	entryPath := filepath.Join(pluginDir, manifest.EntryPoint)

	// Verify entry point exists
	info, err := os.Stat(entryPath)
	if err != nil {
		return nil, fmt.Errorf("entry point %q not found: %w", manifest.EntryPoint, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("entry point %q is a directory, not a file", manifest.EntryPoint)
	}

	// On Unix, check executable permission
	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			return nil, fmt.Errorf("entry point %q is not executable (chmod +x)", manifest.EntryPoint)
		}
	}

	return &Plugin{
		Manifest:  manifest,
		Dir:       pluginDir,
		EntryPath: entryPath,
		Enabled:   false, // set by caller from config
		Config:    make(map[string]string),
	}, nil
}
