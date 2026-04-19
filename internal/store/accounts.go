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

// Account represents a tracked email account.
type Account struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	PlanName  string `json:"planName"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// AllAccounts returns all tracked accounts.
func (s *Store) AllAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`SELECT id, email, plan_name, created_at, updated_at FROM accounts ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		if err := rows.Scan(&a.ID, &a.Email, &a.PlanName, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}
