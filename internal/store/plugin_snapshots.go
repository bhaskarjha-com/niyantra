package store

import "fmt"

// PluginSnapshot represents a captured data point from an external plugin.
type PluginSnapshot struct {
	ID            int64   `json:"id"`
	PluginID      string  `json:"pluginId"`
	Provider      string  `json:"provider"`
	Label         string  `json:"label"`
	Email         string  `json:"email"`
	UsagePct      float64 `json:"usagePct"`
	UsageDisplay  string  `json:"usageDisplay"`
	Plan          string  `json:"plan"`
	ModelsJSON    string  `json:"modelsJson"`
	MetadataJSON  string  `json:"metadataJson"`
	CapturedAt    string  `json:"capturedAt"`
	CaptureMethod string  `json:"captureMethod"`
}

// InsertPluginSnapshot stores a plugin capture result.
func (s *Store) InsertPluginSnapshot(snap *PluginSnapshot) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO plugin_snapshots
			(plugin_id, provider, label, email, usage_pct, usage_display,
			 plan, models_json, metadata_json, capture_method)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, snap.PluginID, snap.Provider, snap.Label, snap.Email,
		snap.UsagePct, snap.UsageDisplay, snap.Plan,
		snap.ModelsJSON, snap.MetadataJSON, snap.CaptureMethod)
	if err != nil {
		return 0, fmt.Errorf("store: insert plugin snapshot: %w", err)
	}
	return res.LastInsertId()
}

// LatestPluginSnapshot returns the most recent snapshot for a given plugin.
func (s *Store) LatestPluginSnapshot(pluginID string) (*PluginSnapshot, error) {
	snap := &PluginSnapshot{}
	err := s.db.QueryRow(`
		SELECT id, plugin_id, provider, label, email, usage_pct, usage_display,
			   plan, models_json, metadata_json, captured_at, capture_method
		FROM plugin_snapshots
		WHERE plugin_id = ?
		ORDER BY captured_at DESC
		LIMIT 1
	`, pluginID).Scan(
		&snap.ID, &snap.PluginID, &snap.Provider, &snap.Label, &snap.Email,
		&snap.UsagePct, &snap.UsageDisplay, &snap.Plan,
		&snap.ModelsJSON, &snap.MetadataJSON, &snap.CapturedAt, &snap.CaptureMethod)
	if err != nil {
		return nil, fmt.Errorf("store: latest plugin snapshot %s: %w", pluginID, err)
	}
	return snap, nil
}

// AllLatestPluginSnapshots returns the most recent snapshot for each plugin.
func (s *Store) AllLatestPluginSnapshots() ([]*PluginSnapshot, error) {
	rows, err := s.db.Query(`
		SELECT ps.id, ps.plugin_id, ps.provider, ps.label, ps.email,
			   ps.usage_pct, ps.usage_display, ps.plan,
			   ps.models_json, ps.metadata_json, ps.captured_at, ps.capture_method
		FROM plugin_snapshots ps
		INNER JOIN (
			SELECT plugin_id, MAX(captured_at) AS max_time
			FROM plugin_snapshots
			GROUP BY plugin_id
		) latest ON ps.plugin_id = latest.plugin_id AND ps.captured_at = latest.max_time
		ORDER BY ps.plugin_id
	`)
	if err != nil {
		return nil, fmt.Errorf("store: all latest plugin snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*PluginSnapshot
	for rows.Next() {
		snap := &PluginSnapshot{}
		if err := rows.Scan(
			&snap.ID, &snap.PluginID, &snap.Provider, &snap.Label, &snap.Email,
			&snap.UsagePct, &snap.UsageDisplay, &snap.Plan,
			&snap.ModelsJSON, &snap.MetadataJSON, &snap.CapturedAt, &snap.CaptureMethod,
		); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}

// PluginSnapshotCount returns the total number of plugin snapshots.
func (s *Store) PluginSnapshotCount() int {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM plugin_snapshots`).Scan(&count)
	return count
}
