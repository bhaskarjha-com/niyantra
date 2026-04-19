package store

import (
	"encoding/json"
	"fmt"
)

// ActivityEntry represents a structured event in the activity log.
type ActivityEntry struct {
	ID           int64  `json:"id"`
	Timestamp    string `json:"timestamp"`
	Level        string `json:"level"`
	Source       string `json:"source"`
	EventType    string `json:"eventType"`
	AccountEmail string `json:"accountEmail"`
	SnapshotID   int64  `json:"snapshotId"`
	Details      string `json:"details"`
}

// LogActivity writes a structured event to the activity log.
func (s *Store) LogActivity(level, source, eventType, email string, snapID int64, details map[string]interface{}) error {
	detailsJSON := "{}"
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailsJSON = string(b)
		}
	}

	_, err := s.db.Exec(`
		INSERT INTO activity_log (level, source, event_type, account_email, snapshot_id, details)
		VALUES (?, ?, ?, ?, ?, ?)
	`, level, source, eventType, email, snapID, detailsJSON)
	if err != nil {
		return fmt.Errorf("store: log activity: %w", err)
	}
	return nil
}

// LogInfo is a convenience wrapper for info-level events.
func (s *Store) LogInfo(source, eventType, email string, details map[string]interface{}) {
	s.LogActivity("info", source, eventType, email, 0, details)
}

// LogInfoSnap logs an info event associated with a snapshot.
func (s *Store) LogInfoSnap(source, eventType, email string, snapID int64, details map[string]interface{}) {
	s.LogActivity("info", source, eventType, email, snapID, details)
}

// LogError is a convenience wrapper for error-level events.
func (s *Store) LogError(source, eventType, email string, details map[string]interface{}) {
	s.LogActivity("error", source, eventType, email, 0, details)
}

// RecentActivity returns the most recent activity log entries.
func (s *Store) RecentActivity(limit int, eventType string) ([]*ActivityEntry, error) {
	query := `SELECT id, timestamp, level, source, event_type, 
		COALESCE(account_email,''), snapshot_id, COALESCE(details,'{}')
		FROM activity_log`
	args := []interface{}{}

	if eventType != "" {
		query += ` WHERE event_type = ?`
		args = append(args, eventType)
	}
	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query activity: %w", err)
	}
	defer rows.Close()

	var entries []*ActivityEntry
	for rows.Next() {
		e := &ActivityEntry{}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Level, &e.Source, &e.EventType,
			&e.AccountEmail, &e.SnapshotID, &e.Details); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
