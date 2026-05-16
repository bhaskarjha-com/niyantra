package store

import "fmt"

// GetOrCreateAccount returns the account ID for the given email and provider,
// creating a new account if one doesn't exist.
// Provider must be one of: "antigravity", "codex", "claude", "cursor", "copilot".
func (s *Store) GetOrCreateAccount(email, planName, provider string) (int64, error) {
	if provider == "" {
		provider = "antigravity"
	}

	// Upsert: insert or update plan_name and updated_at on conflict
	_, err := s.db.Exec(`
		INSERT INTO accounts (email, plan_name, provider)
		VALUES (?, ?, ?)
		ON CONFLICT(email, provider) DO UPDATE SET
			plan_name = excluded.plan_name,
			updated_at = datetime('now')
	`, email, planName, provider)
	if err != nil {
		return 0, fmt.Errorf("store: upsert account: %w", err)
	}

	var id int64
	err = s.db.QueryRow("SELECT id FROM accounts WHERE email = ? AND provider = ?", email, provider).Scan(&id)
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

// Account represents a tracked account (any provider).
type Account struct {
	ID               int64  `json:"id"`
	Email            string `json:"email"`
	PlanName         string `json:"planName"`
	Provider         string `json:"provider"`
	Notes            string `json:"notes"`
	Tags             string `json:"tags"`             // comma-separated: "work,primary"
	PinnedGroup      string `json:"pinnedGroup"`      // for F3: pinned quota group key
	CreditRenewalDay int    `json:"creditRenewalDay"` // day of month (1-31) when AI credits refresh
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// AllAccounts returns all tracked accounts.
func (s *Store) AllAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`SELECT id, email, plan_name, COALESCE(provider,'antigravity'), COALESCE(notes,''), COALESCE(tags,''), COALESCE(pinned_group,''), COALESCE(credit_renewal_day,0), created_at, updated_at FROM accounts ORDER BY provider, email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		if err := rows.Scan(&a.ID, &a.Email, &a.PlanName, &a.Provider, &a.Notes, &a.Tags, &a.PinnedGroup, &a.CreditRenewalDay, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// GetAccountByID returns a single account by primary key.
func (s *Store) GetAccountByID(id int64) (*Account, error) {
	a := &Account{}
	err := s.db.QueryRow(
		`SELECT id, email, plan_name, COALESCE(provider,'antigravity'),
			COALESCE(notes,''), COALESCE(tags,''), COALESCE(pinned_group,''),
			COALESCE(credit_renewal_day,0), created_at, updated_at
		FROM accounts WHERE id = ?`, id,
	).Scan(&a.ID, &a.Email, &a.PlanName, &a.Provider, &a.Notes, &a.Tags,
		&a.PinnedGroup, &a.CreditRenewalDay, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("store: account %d: %w", id, err)
	}
	return a, nil
}

// AccountMeta returns the notes, tags, pinned_group, and credit_renewal_day for a specific account.
func (s *Store) AccountMeta(accountID int64) (notes, tags, pinnedGroup string, creditRenewalDay int, err error) {
	err = s.db.QueryRow(
		`SELECT COALESCE(notes,''), COALESCE(tags,''), COALESCE(pinned_group,''), COALESCE(credit_renewal_day,0) FROM accounts WHERE id = ?`,
		accountID,
	).Scan(&notes, &tags, &pinnedGroup, &creditRenewalDay)
	return
}

// UpdateAccountMeta updates notes, tags, pinned_group, and credit_renewal_day for an account.
func (s *Store) UpdateAccountMeta(accountID int64, notes, tags, pinnedGroup string, creditRenewalDay int) error {
	_, err := s.db.Exec(
		`UPDATE accounts SET notes = ?, tags = ?, pinned_group = ?, credit_renewal_day = ?, updated_at = datetime('now') WHERE id = ?`,
		notes, tags, pinnedGroup, creditRenewalDay, accountID,
	)
	return err
}

// DeleteAccount removes an account and all its associated data (snapshots, cycles, sessions, codex/cursor snapshots).
// Returns the total number of deleted rows across all tables.
func (s *Store) DeleteAccount(accountID int64) (int64, error) {
	var totalDeleted int64

	// Delete in dependency order: children first
	tables := []struct {
		query string
		col   string
	}{
		{"DELETE FROM snapshots WHERE account_id = ?", "snapshots"},
		{"DELETE FROM antigravity_reset_cycles WHERE account_id = ?", "reset_cycles"},
		{"DELETE FROM codex_snapshots WHERE account_id = ?", "codex_snapshots"},
		{"DELETE FROM cursor_snapshots WHERE account_id = ?", "cursor_snapshots"},
		{"DELETE FROM copilot_snapshots WHERE account_id = ?", "copilot_snapshots"},
		{"DELETE FROM accounts WHERE id = ?", "accounts"},
	}

	for _, t := range tables {
		result, err := s.db.Exec(t.query, accountID)
		if err != nil {
			return totalDeleted, fmt.Errorf("store: delete %s for account %d: %w", t.col, accountID, err)
		}
		n, _ := result.RowsAffected()
		totalDeleted += n
	}

	return totalDeleted, nil
}

// DeleteAccountSnapshots removes all snapshots for a specific account but keeps the account itself.
func (s *Store) DeleteAccountSnapshots(accountID int64) (int64, error) {
	result, err := s.db.Exec("DELETE FROM snapshots WHERE account_id = ?", accountID)
	if err != nil {
		return 0, fmt.Errorf("store: delete snapshots for account %d: %w", accountID, err)
	}
	return result.RowsAffected()
}

// DeleteSnapshot removes a single snapshot by ID.
func (s *Store) DeleteSnapshot(snapshotID int64) error {
	result, err := s.db.Exec("DELETE FROM snapshots WHERE id = ?", snapshotID)
	if err != nil {
		return fmt.Errorf("store: delete snapshot %d: %w", snapshotID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("store: snapshot %d not found", snapshotID)
	}
	return nil
}
