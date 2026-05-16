package agent

import (
	"context"
	"encoding/json"

	"github.com/bhaskarjha-com/niyantra/internal/plugin"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

// pollPlugins runs all enabled plugins and stores their capture results.
func (a *PollingAgent) pollPlugins(ctx context.Context) {
	if a.plugins == nil {
		return
	}

	for _, p := range a.plugins {
		if !p.Enabled {
			continue
		}

		result, err := p.Run(ctx, a.logger)
		if err != nil {
			a.logger.Warn("Plugin capture failed",
				"plugin", p.Manifest.ID,
				"error", err)
			continue
		}

		if result.Status != "ok" {
			a.logger.Warn("Plugin returned error",
				"plugin", p.Manifest.ID,
				"error", result.Error)
			continue
		}

		// Marshal models and metadata to JSON strings
		modelsJSON := "[]"
		if result.Data.Models != nil {
			if b, err := json.Marshal(result.Data.Models); err == nil {
				modelsJSON = string(b)
			}
		}
		metadataJSON := "{}"
		if result.Data.Metadata != nil {
			if b, err := json.Marshal(result.Data.Metadata); err == nil {
				metadataJSON = string(b)
			}
		}

		snap := &store.PluginSnapshot{
			PluginID:      p.Manifest.ID,
			Provider:      result.Data.Provider,
			Label:         result.Data.Label,
			Email:         result.Data.Email,
			UsagePct:      result.Data.UsagePct,
			UsageDisplay:  result.Data.UsageDisplay,
			Plan:          result.Data.Plan,
			ModelsJSON:    modelsJSON,
			MetadataJSON:  metadataJSON,
			CaptureMethod: "plugin",
		}

		if _, err := a.store.InsertPluginSnapshot(snap); err != nil {
			a.logger.Error("Failed to store plugin snapshot",
				"plugin", p.Manifest.ID,
				"error", err)
			continue
		}

		a.store.UpdateSourceCapture("plugin_" + p.Manifest.ID)

		// Check quota thresholds via notifier
		if a.notifier != nil && result.Data.UsagePct > 0 {
			a.notifier.CheckClaudeQuota("plugin_"+p.Manifest.ID, result.Data.UsagePct)
		}

		a.logger.Info("Plugin capture complete",
			"plugin", p.Manifest.ID,
			"provider", result.Data.Provider,
			"usagePct", result.Data.UsagePct)
	}
}

// SetPlugins sets the list of discovered plugins for the agent to poll.
func (a *PollingAgent) SetPlugins(plugins []*plugin.Plugin) {
	a.plugins = plugins
}
