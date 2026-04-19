package store

import (
	"database/sql"
	"fmt"
	"time"
)

// UsageLog represents a manual usage log entry for a subscription.
type UsageLog struct {
	ID             int64     `json:"id"`
	SubscriptionID int64     `json:"subscriptionId"`
	LoggedAt       time.Time `json:"loggedAt"`
	UsageAmount    float64   `json:"usageAmount"`
	UsageUnit      string    `json:"usageUnit"`
	Notes          string    `json:"notes"`
}

// UsageLogSummary provides aggregated stats for a subscription's usage.
type UsageLogSummary struct {
	TotalAmount float64    `json:"totalAmount"`
	LogCount    int        `json:"logCount"`
	LastUnit    string     `json:"lastUnit"`
	LastLogged  *time.Time `json:"lastLogged"`
}

// InsertUsageLog creates a new usage log entry.
func (s *Store) InsertUsageLog(subID int64, amount float64, unit, notes string) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO usage_logs (subscription_id, usage_amount, usage_unit, notes)
		VALUES (?, ?, ?, ?)`, subID, amount, unit, notes)
	if err != nil {
		return 0, fmt.Errorf("store.InsertUsageLog: %w", err)
	}
	return res.LastInsertId()
}

// UsageLogsForSubscription returns usage logs for a subscription, newest first.
func (s *Store) UsageLogsForSubscription(subID int64, limit int) ([]*UsageLog, error) {
	rows, err := s.db.Query(`
		SELECT id, subscription_id, logged_at, usage_amount, usage_unit, notes
		FROM usage_logs
		WHERE subscription_id = ?
		ORDER BY logged_at DESC
		LIMIT ?`, subID, limit)
	if err != nil {
		return nil, fmt.Errorf("store.UsageLogsForSubscription: %w", err)
	}
	defer rows.Close()

	var logs []*UsageLog
	for rows.Next() {
		log := &UsageLog{}
		var loggedAt string
		if err := rows.Scan(&log.ID, &log.SubscriptionID, &loggedAt,
			&log.UsageAmount, &log.UsageUnit, &log.Notes); err != nil {
			return nil, err
		}
		log.LoggedAt, _ = time.Parse(time.RFC3339, loggedAt)
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// UsageLogSummaryFor returns aggregated usage stats for a subscription.
func (s *Store) UsageLogSummaryFor(subID int64) (*UsageLogSummary, error) {
	row := s.db.QueryRow(`
		SELECT COALESCE(SUM(usage_amount), 0), COUNT(*),
		       COALESCE((SELECT usage_unit FROM usage_logs WHERE subscription_id = ? ORDER BY logged_at DESC LIMIT 1), ''),
		       (SELECT logged_at FROM usage_logs WHERE subscription_id = ? ORDER BY logged_at DESC LIMIT 1)
		FROM usage_logs WHERE subscription_id = ?`, subID, subID, subID)

	summary := &UsageLogSummary{}
	var lastLogged sql.NullString
	if err := row.Scan(&summary.TotalAmount, &summary.LogCount, &summary.LastUnit, &lastLogged); err != nil {
		return nil, fmt.Errorf("store.UsageLogSummaryFor: %w", err)
	}
	if lastLogged.Valid {
		t, _ := time.Parse(time.RFC3339, lastLogged.String)
		summary.LastLogged = &t
	}
	return summary, nil
}

// DeleteUsageLog removes a usage log entry by ID.
func (s *Store) DeleteUsageLog(id int64) error {
	res, err := s.db.Exec(`DELETE FROM usage_logs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteUsageLog: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("store.DeleteUsageLog: not found")
	}
	return nil
}
