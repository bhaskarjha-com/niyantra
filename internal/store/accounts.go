package store

import "fmt"

// GetOrCreateAccount returns the account ID for the given email,
// creating a new account if one doesn't exist.
func (s *Store) GetOrCreateAccount(email, planName string) (int64, error) {
	// Upsert: insert or update plan_name and updated_at on conflict
	_, err := s.db.Exec(`
		INSERT INTO accounts (email, plan_name)
		VALUES (?, ?)
		ON CONFLICT(email) DO UPDATE SET
			plan_name = excluded.plan_name,
			updated_at = datetime('now')
	`, email, planName)
	if err != nil {
		return 0, fmt.Errorf("store: upsert account: %w", err)
	}

	var id int64
	err = s.db.QueryRow("SELECT id FROM accounts WHERE email = ?", email).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: get account id: %w", err)
	}

	return id, nil
}

// AccountCount returns the total number of tracked accounts.
func (s *Store) AccountCount() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	return count
}
