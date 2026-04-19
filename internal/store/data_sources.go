package store

import "fmt"

// DataSource represents a registered data source.
type DataSource struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SourceType   string `json:"sourceType"`
	Enabled      bool   `json:"enabled"`
	ConfigJSON   string `json:"configJson"`
	LastCapture  string `json:"lastCapture"`
	CaptureCount int64  `json:"captureCount"`
	CreatedAt    string `json:"createdAt"`
}

// AllDataSources returns all registered data sources.
func (s *Store) AllDataSources() ([]*DataSource, error) {
	rows, err := s.db.Query(`
		SELECT id, name, source_type, enabled, COALESCE(config_json,'{}'),
			COALESCE(last_capture,''), capture_count, created_at
		FROM data_sources ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("store: query data sources: %w", err)
	}
	defer rows.Close()

	var sources []*DataSource
	for rows.Next() {
		ds := &DataSource{}
		var enabled int
		if err := rows.Scan(&ds.ID, &ds.Name, &ds.SourceType, &enabled,
			&ds.ConfigJSON, &ds.LastCapture, &ds.CaptureCount, &ds.CreatedAt); err != nil {
			return nil, err
		}
		ds.Enabled = enabled == 1
		sources = append(sources, ds)
	}
	return sources, nil
}

// UpdateSourceCapture updates last_capture time and increments capture_count.
func (s *Store) UpdateSourceCapture(sourceID string) error {
	_, err := s.db.Exec(`
		UPDATE data_sources
		SET last_capture = datetime('now'), capture_count = capture_count + 1
		WHERE id = ?
	`, sourceID)
	if err != nil {
		return fmt.Errorf("store: update source capture %s: %w", sourceID, err)
	}
	return nil
}

// SetSourceEnabled enables or disables a data source.
func (s *Store) SetSourceEnabled(sourceID string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec(`UPDATE data_sources SET enabled = ? WHERE id = ?`, val, sourceID)
	if err != nil {
		return fmt.Errorf("store: set source enabled %s: %w", sourceID, err)
	}
	return nil
}
