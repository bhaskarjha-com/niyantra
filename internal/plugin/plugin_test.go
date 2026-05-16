package plugin

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		wantErr bool
	}{
		{
			name:    "valid manifest",
			m:       Manifest{ID: "test-plugin", Name: "Test Plugin", EntryPoint: "capture.py"},
			wantErr: false,
		},
		{
			name:    "missing id",
			m:       Manifest{Name: "Test", EntryPoint: "capture.py"},
			wantErr: true,
		},
		{
			name:    "missing name",
			m:       Manifest{ID: "test", EntryPoint: "capture.py"},
			wantErr: true,
		},
		{
			name:    "missing entryPoint",
			m:       Manifest{ID: "test", Name: "Test"},
			wantErr: true,
		},
		{
			name:    "path traversal in entryPoint",
			m:       Manifest{ID: "test", Name: "Test", EntryPoint: "../../../etc/passwd"},
			wantErr: true,
		},
		{
			name:    "absolute path in entryPoint",
			m:       Manifest{ID: "test", Name: "Test", EntryPoint: "/usr/bin/evil"},
			wantErr: true,
		},
		{
			name:    "invalid chars in id",
			m:       Manifest{ID: "test plugin!", Name: "Test", EntryPoint: "capture.py"},
			wantErr: true,
		},
		{
			name:    "negative timeout",
			m:       Manifest{ID: "test", Name: "Test", EntryPoint: "capture.py", Timeout: -1},
			wantErr: true,
		},
		{
			name:    "zero timeout is valid (uses default)",
			m:       Manifest{ID: "test", Name: "Test", EntryPoint: "capture.py", Timeout: 0},
			wantErr: false,
		},
		{
			name:    "hyphen and underscore in id are valid",
			m:       Manifest{ID: "my-custom_plugin-v2", Name: "My Plugin", EntryPoint: "run.sh"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEffectiveTimeout(t *testing.T) {
	m := Manifest{Timeout: 0}
	if m.EffectiveTimeout() != 30 {
		t.Errorf("expected default 30, got %d", m.EffectiveTimeout())
	}

	m.Timeout = 60
	if m.EffectiveTimeout() != 60 {
		t.Errorf("expected 60, got %d", m.EffectiveTimeout())
	}
}

func TestDiscoverEmptyDir(t *testing.T) {
	dir := t.TempDir()
	plugins, errs := Discover(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestDiscoverNonExistentDir(t *testing.T) {
	plugins, errs := Discover(filepath.Join(t.TempDir(), "nonexistent"))
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for nonexistent dir, got %d", len(errs))
	}
}

func TestDiscoverValidPlugin(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "test-plugin")
	os.MkdirAll(pluginDir, 0755)

	// Create a valid plugin.json
	manifest := Manifest{
		ID:         "test-plugin",
		Name:       "Test Plugin",
		Version:    "1.0.0",
		EntryPoint: "capture.py",
		Config: map[string]ConfigField{
			"api_key": {Type: "string", Label: "API Key", Required: true, Secret: true},
		},
	}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0644)

	// Create the entry point script
	scriptContent := "#!/usr/bin/env python3\nimport json, sys\nprint(json.dumps({\"status\":\"ok\"}))\n"
	scriptPath := filepath.Join(pluginDir, "capture.py")
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	plugins, errs := Discover(dir)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Manifest.ID != "test-plugin" {
		t.Errorf("expected id 'test-plugin', got %q", plugins[0].Manifest.ID)
	}
	if plugins[0].Manifest.Name != "Test Plugin" {
		t.Errorf("expected name 'Test Plugin', got %q", plugins[0].Manifest.Name)
	}
	if plugins[0].Dir != pluginDir {
		t.Errorf("expected dir %q, got %q", pluginDir, plugins[0].Dir)
	}
}

func TestDiscoverSkipsInvalidPlugin(t *testing.T) {
	dir := t.TempDir()

	// Valid plugin
	validDir := filepath.Join(dir, "valid")
	os.MkdirAll(validDir, 0755)
	validManifest, _ := json.Marshal(Manifest{ID: "valid", Name: "Valid", EntryPoint: "run.py"})
	os.WriteFile(filepath.Join(validDir, "plugin.json"), validManifest, 0644)
	os.WriteFile(filepath.Join(validDir, "run.py"), []byte("#!/usr/bin/env python3\n"), 0755)

	// Invalid plugin (bad JSON)
	invalidDir := filepath.Join(dir, "invalid")
	os.MkdirAll(invalidDir, 0755)
	os.WriteFile(filepath.Join(invalidDir, "plugin.json"), []byte("{bad json}"), 0644)

	// Invalid plugin (missing entry point)
	missingDir := filepath.Join(dir, "missing-entry")
	os.MkdirAll(missingDir, 0755)
	missingManifest, _ := json.Marshal(Manifest{ID: "missing", Name: "Missing", EntryPoint: "nonexistent.py"})
	os.WriteFile(filepath.Join(missingDir, "plugin.json"), missingManifest, 0644)

	plugins, errs := Discover(dir)

	if len(plugins) != 1 {
		t.Errorf("expected 1 valid plugin, got %d", len(plugins))
	}
	if len(errs) != 2 {
		t.Errorf("expected 2 errors (bad json + missing entry), got %d: %v", len(errs), errs)
	}
}

func TestPluginRunSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping subprocess test on Windows CI (no python3)")
	}

	dir := t.TempDir()

	// Create a minimal Python plugin that returns valid JSON
	script := `#!/usr/bin/env python3
import json, sys
req = json.load(sys.stdin)
json.dump({
    "status": "ok",
    "data": {
        "provider": "test",
        "label": "Test Provider",
        "usage_pct": 42.5,
        "usage_display": "42.5%",
        "plan": "Free",
        "models": [],
        "metadata": {"key": "value"}
    }
}, sys.stdout)
`
	scriptPath := filepath.Join(dir, "capture.py")
	os.WriteFile(scriptPath, []byte(script), 0755)

	p := &Plugin{
		Manifest: Manifest{
			ID:         "test",
			Name:       "Test",
			EntryPoint: "capture.py",
			Timeout:    5,
		},
		Dir:       dir,
		EntryPath: scriptPath,
		Config:    map[string]string{"key": "value"},
	}

	logger := slog.Default()
	result, err := p.Run(context.Background(), logger)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", result.Status)
	}
	if result.Data.UsagePct != 42.5 {
		t.Errorf("expected usage_pct 42.5, got %f", result.Data.UsagePct)
	}
	if result.Data.Provider != "test" {
		t.Errorf("expected provider 'test', got %q", result.Data.Provider)
	}
}

func TestPluginRunError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping subprocess test on Windows CI (no python3)")
	}

	dir := t.TempDir()

	script := `#!/usr/bin/env python3
import json, sys
req = json.load(sys.stdin)
json.dump({"status": "error", "error": "API key invalid"}, sys.stdout)
`
	scriptPath := filepath.Join(dir, "capture.py")
	os.WriteFile(scriptPath, []byte(script), 0755)

	p := &Plugin{
		Manifest:  Manifest{ID: "test", Name: "Test", EntryPoint: "capture.py", Timeout: 5},
		Dir:       dir,
		EntryPath: scriptPath,
		Config:    map[string]string{},
	}

	result, err := p.Run(context.Background(), slog.Default())
	if err != nil {
		t.Fatalf("Run() should not return Go error for plugin-level errors: %v", err)
	}
	if result.Status != "error" {
		t.Errorf("expected status 'error', got %q", result.Status)
	}
	if result.Error != "API key invalid" {
		t.Errorf("expected error 'API key invalid', got %q", result.Error)
	}
}

func TestPluginRunTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping subprocess test on Windows CI (no python3)")
	}

	dir := t.TempDir()

	// Plugin that sleeps forever
	script := `#!/usr/bin/env python3
import time, sys, json
req = json.load(sys.stdin)
time.sleep(60)
`
	scriptPath := filepath.Join(dir, "slow.py")
	os.WriteFile(scriptPath, []byte(script), 0755)

	p := &Plugin{
		Manifest:  Manifest{ID: "slow", Name: "Slow", EntryPoint: "slow.py", Timeout: 1},
		Dir:       dir,
		EntryPath: scriptPath,
		Config:    map[string]string{},
	}

	_, err := p.Run(context.Background(), slog.Default())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// Should contain "timed out" or context deadline
	if !contains(err.Error(), "timed out") && !contains(err.Error(), "killed") && !contains(err.Error(), "signal") {
		t.Errorf("expected timeout-related error, got: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
