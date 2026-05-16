package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bhaskarjha-com/niyantra/internal/plugin"
)

// handlePlugins returns all discovered plugins with their status.
// GET /api/plugins
func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	pluginsDir := plugin.DefaultPluginsDir()
	plugins, errs := plugin.Discover(pluginsDir)

	// Load enabled state and config for each plugin from SQLite
	type pluginInfo struct {
		plugin.Manifest `json:"manifest"`
		Dir             string            `json:"dir"`
		Enabled         bool              `json:"enabled"`
		Config          map[string]string `json:"config"`
		LastCapture     string            `json:"lastCapture,omitempty"`
		CaptureCount    int64             `json:"captureCount"`
	}

	var result []pluginInfo
	for _, p := range plugins {
		info := pluginInfo{
			Manifest: p.Manifest,
			Dir:      p.Dir,
			Config:   make(map[string]string),
		}

		// Check if plugin is enabled in config
		info.Enabled = s.store.GetConfigBool("plugin_" + p.Manifest.ID + "_enabled")

		// Load plugin-specific config values
		for key, field := range p.Manifest.Config {
			val := s.store.GetConfig("plugin_" + p.Manifest.ID + "_" + key)
			if val == "" && field.Default != "" {
				val = field.Default
			}
			// Mask secret values
			if field.Secret && val != "" {
				info.Config[key] = "••••••••"
			} else {
				info.Config[key] = val
			}
		}

		// Get latest snapshot info from data_sources
		ds, _ := s.store.AllDataSources()
		for _, d := range ds {
			if d.ID == "plugin_"+p.Manifest.ID {
				info.LastCapture = d.LastCapture
				info.CaptureCount = d.CaptureCount
				break
			}
		}

		result = append(result, info)
	}

	var discoveryErrors []string
	for _, e := range errs {
		discoveryErrors = append(discoveryErrors, e.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"plugins":    result,
		"pluginsDir": pluginsDir,
		"errors":     discoveryErrors,
	})
}

// handlePluginStatus returns the latest snapshot for a specific plugin.
// GET /api/plugins/{id}/status
func (s *Server) handlePluginStatus(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")
	if pluginID == "" {
		http.Error(w, `{"error":"plugin id required"}`, http.StatusBadRequest)
		return
	}

	snap, err := s.store.LatestPluginSnapshot(pluginID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"pluginId": pluginID,
			"status":   "no_data",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"pluginId": pluginID,
		"status":   "ok",
		"snapshot": snap,
	})
}

// handlePluginRun manually triggers a plugin capture and returns the result.
// POST /api/plugins/{id}/run
func (s *Server) handlePluginRun(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")
	if pluginID == "" {
		http.Error(w, `{"error":"plugin id required"}`, http.StatusBadRequest)
		return
	}

	// Discover and find the specific plugin
	pluginsDir := plugin.DefaultPluginsDir()
	plugins, _ := plugin.Discover(pluginsDir)

	var target *plugin.Plugin
	for _, p := range plugins {
		if p.Manifest.ID == pluginID {
			target = p
			break
		}
	}

	if target == nil {
		http.Error(w, `{"error":"plugin not found"}`, http.StatusNotFound)
		return
	}

	// Load config values from SQLite
	for key := range target.Manifest.Config {
		val := s.store.GetConfig("plugin_" + pluginID + "_" + key)
		if val != "" {
			target.Config[key] = val
		}
	}

	// Execute the plugin
	result, err := target.Run(r.Context(), s.logger)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handlePluginConfig saves plugin configuration values.
// PUT /api/plugins/{id}/config
func (s *Server) handlePluginConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")
	if pluginID == "" {
		http.Error(w, `{"error":"plugin id required"}`, http.StatusBadRequest)
		return
	}

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	// Validate: only save keys that exist in the plugin manifest
	pluginsDir := plugin.DefaultPluginsDir()
	plugins, _ := plugin.Discover(pluginsDir)

	var target *plugin.Plugin
	for _, p := range plugins {
		if p.Manifest.ID == pluginID {
			target = p
			break
		}
	}

	if target == nil {
		http.Error(w, `{"error":"plugin not found"}`, http.StatusNotFound)
		return
	}

	// Handle special "enabled" key
	if enabled, ok := body["enabled"]; ok {
		s.store.SetConfig("plugin_"+pluginID+"_enabled", enabled)
		delete(body, "enabled")

		// Register/update data source
		if strings.EqualFold(enabled, "true") {
			s.registerPluginDataSource(target)
		}
	}

	// Save each config key
	for key, val := range body {
		if _, exists := target.Manifest.Config[key]; !exists {
			continue // skip unknown keys
		}
		configKey := "plugin_" + pluginID + "_" + key
		s.store.SetConfig(configKey, val)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// registerPluginDataSource ensures a data_sources entry exists for this plugin.
func (s *Server) registerPluginDataSource(p *plugin.Plugin) {
	sourceID := "plugin_" + p.Manifest.ID
	// Use INSERT OR IGNORE to avoid duplicates
	s.store.ExecRaw(`
		INSERT OR IGNORE INTO data_sources (id, name, source_type, enabled, config_json)
		VALUES (?, ?, 'plugin', 1, '{}')
	`, sourceID, p.Manifest.Name)
}
