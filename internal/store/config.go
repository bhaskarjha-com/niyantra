package store

import (
	"fmt"
	"strconv"
)

// ConfigEntry represents a server-level configuration entry.
type ConfigEntry struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"valueType"`
	Category    string `json:"category"`
	Label       string `json:"label"`
	Description string `json:"description"`
	UpdatedAt   string `json:"updatedAt"`
}

// GetConfig returns a single config value as string.
func (s *Store) GetConfig(key string) string {
	var val string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

// GetConfigInt returns a config value as int with a default fallback.
func (s *Store) GetConfigInt(key string, defaultVal int) int {
	val := s.GetConfig(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// GetConfigFloat returns a config value as float64 with a default fallback.
func (s *Store) GetConfigFloat(key string, defaultVal float64) float64 {
	val := s.GetConfig(key)
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// GetConfigBool returns a config value as bool.
func (s *Store) GetConfigBool(key string) bool {
	return s.GetConfig(key) == "true"
}

// SetConfig updates a config value and returns the old value.
func (s *Store) SetConfig(key, value string) (string, error) {
	oldVal := s.GetConfig(key)

	_, err := s.db.Exec(`
		UPDATE config SET value = ?, updated_at = datetime('now')
		WHERE key = ?
	`, value, key)
	if err != nil {
		return "", fmt.Errorf("store: update config %s: %w", key, err)
	}

	return oldVal, nil
}

// AllConfig returns all config entries, optionally filtered by category.
func (s *Store) AllConfig(category string) ([]*ConfigEntry, error) {
	query := `SELECT key, value, value_type, category, label, COALESCE(description,''), updated_at FROM config`
	args := []interface{}{}

	if category != "" {
		query += ` WHERE category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY category, key`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query config: %w", err)
	}
	defer rows.Close()

	var entries []*ConfigEntry
	for rows.Next() {
		e := &ConfigEntry{}
		if err := rows.Scan(&e.Key, &e.Value, &e.ValueType, &e.Category, &e.Label, &e.Description, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ConfigMap returns all config as a key→value map for fast lookups.
func (s *Store) ConfigMap() map[string]string {
	m := make(map[string]string)
	rows, err := s.db.Query(`SELECT key, value FROM config`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			m[k] = v
		}
	}
	return m
}
