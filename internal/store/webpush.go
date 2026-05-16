package store

// ── F19: WebPush Subscription Storage ───────────────────────────

// WebPushSubscription represents a stored browser push subscription.
type WebPushSubscription struct {
	ID        int    `json:"id"`
	Endpoint  string `json:"endpoint"`
	KeyP256dh string `json:"key_p256dh"`
	KeyAuth   string `json:"key_auth"`
	CreatedAt string `json:"created_at"`
}

// SaveWebPushSubscription stores or updates a browser push subscription.
// Uses INSERT OR REPLACE on the unique endpoint constraint.
func (s *Store) SaveWebPushSubscription(endpoint, p256dh, auth string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO webpush_subscriptions (endpoint, key_p256dh, key_auth, created_at)
		VALUES (?, ?, ?, datetime('now'))
	`, endpoint, p256dh, auth)
	return err
}

// DeleteWebPushSubscription removes a subscription by endpoint.
func (s *Store) DeleteWebPushSubscription(endpoint string) error {
	_, err := s.db.Exec(`DELETE FROM webpush_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

// GetWebPushSubscriptions returns all stored push subscriptions.
func (s *Store) GetWebPushSubscriptions() []WebPushSubscription {
	rows, err := s.db.Query(`
		SELECT id, endpoint, key_p256dh, key_auth, created_at
		FROM webpush_subscriptions
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var subs []WebPushSubscription
	for rows.Next() {
		var sub WebPushSubscription
		if err := rows.Scan(&sub.ID, &sub.Endpoint, &sub.KeyP256dh, &sub.KeyAuth, &sub.CreatedAt); err != nil {
			continue
		}
		subs = append(subs, sub)
	}
	return subs
}

// WebPushSubscriptionCount returns the number of stored subscriptions.
func (s *Store) WebPushSubscriptionCount() int {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM webpush_subscriptions`).Scan(&count)
	return count
}
