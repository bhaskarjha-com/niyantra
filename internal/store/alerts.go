package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

// SystemAlert represents a persistent, dismissible system notification.
type SystemAlert struct {
	ID          int64      `json:"id"`
	AlertType   string     `json:"alertType"`
	Severity    string     `json:"severity"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	ContextJSON string     `json:"contextJson,omitempty"`
	Dismissed   bool       `json:"dismissed"`
	CreatedAt   time.Time  `json:"createdAt"`
	DismissedAt *time.Time `json:"dismissedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

// CreateAlert creates a new system alert with deduplication.
// If an active (non-dismissed) alert of the same type already exists,
// its message and timestamp are updated instead of creating a duplicate.
func (s *Store) CreateAlert(alertType, severity, title, message string, ctx map[string]interface{}) (int64, error) {
	// Dedup: check for existing active alert of same type
	var existingID int64
	err := s.db.QueryRow(
		`SELECT id FROM system_alerts WHERE alert_type = ? AND dismissed = 0 LIMIT 1`,
		alertType,
	).Scan(&existingID)

	if err == nil {
		// Update existing alert
		ctxJSON := "{}"
		if ctx != nil {
			if b, err := json.Marshal(ctx); err == nil {
				ctxJSON = string(b)
			}
		}

		// Refresh the expiry for info alerts
		var expiresAt interface{} = nil
		if severity == "info" {
			expiresAt = time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05")
		}

		_, err = s.db.Exec(
			`UPDATE system_alerts SET message = ?, context_json = ?, created_at = datetime('now'), expires_at = ? WHERE id = ?`,
			message, ctxJSON, expiresAt, existingID,
		)
		return existingID, err
	}

	// Create new alert
	ctxJSON := "{}"
	if ctx != nil {
		if b, err := json.Marshal(ctx); err == nil {
			ctxJSON = string(b)
		}
	}

	// Info alerts auto-expire after 24h; critical/warning persist
	var expiresAt interface{} = nil
	if severity == "info" {
		expiresAt = time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	}

	result, err := s.db.Exec(
		`INSERT INTO system_alerts (alert_type, severity, title, message, context_json, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		alertType, severity, title, message, ctxJSON, expiresAt,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ActiveAlerts returns all non-dismissed, non-expired alerts ordered by severity then time.
func (s *Store) ActiveAlerts() ([]*SystemAlert, error) {
	rows, err := s.db.Query(`
		SELECT id, alert_type, severity, title, message, context_json, dismissed, created_at, dismissed_at, expires_at
		FROM system_alerts
		WHERE dismissed = 0
		  AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY
			CASE severity WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END,
			created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*SystemAlert
	for rows.Next() {
		a := &SystemAlert{}
		var dismissedAt, expiresAt sql.NullString
		var createdStr string
		var dismissed int

		if err := rows.Scan(&a.ID, &a.AlertType, &a.Severity, &a.Title, &a.Message,
			&a.ContextJSON, &dismissed, &createdStr, &dismissedAt, &expiresAt); err != nil {
			continue
		}

		a.Dismissed = dismissed != 0
		if t, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
			a.CreatedAt = t
		}
		if dismissedAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", dismissedAt.String); err == nil {
				a.DismissedAt = &t
			}
		}
		if expiresAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", expiresAt.String); err == nil {
				a.ExpiresAt = &t
			}
		}

		alerts = append(alerts, a)
	}
	return alerts, nil
}

// DismissAlert marks a single alert as dismissed.
func (s *Store) DismissAlert(id int64) error {
	_, err := s.db.Exec(
		`UPDATE system_alerts SET dismissed = 1, dismissed_at = datetime('now') WHERE id = ?`,
		id,
	)
	return err
}

// DismissAlertsByType marks all active alerts of a given type as dismissed.
func (s *Store) DismissAlertsByType(alertType string) error {
	_, err := s.db.Exec(
		`UPDATE system_alerts SET dismissed = 1, dismissed_at = datetime('now')
		 WHERE alert_type = ? AND dismissed = 0`,
		alertType,
	)
	return err
}

// CleanupExpiredAlerts removes dismissed alerts older than 7 days and expired alerts.
func (s *Store) CleanupExpiredAlerts() error {
	_, err := s.db.Exec(`
		DELETE FROM system_alerts
		WHERE (dismissed = 1 AND dismissed_at < datetime('now', '-7 days'))
		   OR (expires_at IS NOT NULL AND expires_at < datetime('now'))
	`)
	return err
}
