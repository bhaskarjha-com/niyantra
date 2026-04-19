package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Subscription represents a manually-tracked AI subscription.
type Subscription struct {
	ID            int64   `json:"id"`
	Platform      string  `json:"platform"`
	Category      string  `json:"category"`
	IconKey       string  `json:"iconKey"`
	Email         string  `json:"email"`
	PlanName      string  `json:"planName"`
	Status        string  `json:"status"`
	CostAmount    float64 `json:"costAmount"`
	CostCurrency  string  `json:"costCurrency"`
	BillingCycle  string  `json:"billingCycle"`
	TokenLimit    int64   `json:"tokenLimit"`
	CreditLimit   int64   `json:"creditLimit"`
	RequestLimit  int64   `json:"requestLimit"`
	LimitPeriod   string  `json:"limitPeriod"`
	LimitNote     string  `json:"limitNote"`
	NextRenewal   string  `json:"nextRenewal"`
	StartedAt     string  `json:"startedAt"`
	TrialEndsAt   string  `json:"trialEndsAt"`
	Notes         string  `json:"notes"`
	URL           string  `json:"url"`
	StatusPageURL string  `json:"statusPageUrl"`
	AutoTracked   bool    `json:"autoTracked"`
	AccountID     int64   `json:"accountId"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

// InsertSubscription creates a new subscription record.
func (s *Store) InsertSubscription(sub *Subscription) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO subscriptions (
			platform, category, icon_key, email, plan_name, status,
			cost_amount, cost_currency, billing_cycle,
			token_limit, credit_limit, request_limit, limit_period, limit_note,
			next_renewal, started_at, trial_ends_at,
			notes, url, status_page_url,
			auto_tracked, account_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		sub.Platform, sub.Category, sub.IconKey, sub.Email, sub.PlanName, sub.Status,
		sub.CostAmount, sub.CostCurrency, sub.BillingCycle,
		sub.TokenLimit, sub.CreditLimit, sub.RequestLimit, sub.LimitPeriod, sub.LimitNote,
		sub.NextRenewal, sub.StartedAt, sub.TrialEndsAt,
		sub.Notes, sub.URL, sub.StatusPageURL,
		boolToInt(sub.AutoTracked), sub.AccountID,
	)
	if err != nil {
		return 0, fmt.Errorf("store: insert subscription: %w", err)
	}
	return result.LastInsertId()
}

// UpdateSubscription updates an existing subscription.
func (s *Store) UpdateSubscription(sub *Subscription) error {
	_, err := s.db.Exec(`
		UPDATE subscriptions SET
			platform = ?, category = ?, icon_key = ?, email = ?, plan_name = ?, status = ?,
			cost_amount = ?, cost_currency = ?, billing_cycle = ?,
			token_limit = ?, credit_limit = ?, request_limit = ?, limit_period = ?, limit_note = ?,
			next_renewal = ?, started_at = ?, trial_ends_at = ?,
			notes = ?, url = ?, status_page_url = ?,
			auto_tracked = ?, account_id = ?,
			updated_at = datetime('now')
		WHERE id = ?
	`,
		sub.Platform, sub.Category, sub.IconKey, sub.Email, sub.PlanName, sub.Status,
		sub.CostAmount, sub.CostCurrency, sub.BillingCycle,
		sub.TokenLimit, sub.CreditLimit, sub.RequestLimit, sub.LimitPeriod, sub.LimitNote,
		sub.NextRenewal, sub.StartedAt, sub.TrialEndsAt,
		sub.Notes, sub.URL, sub.StatusPageURL,
		boolToInt(sub.AutoTracked), sub.AccountID,
		sub.ID,
	)
	if err != nil {
		return fmt.Errorf("store: update subscription: %w", err)
	}
	return nil
}

// DeleteSubscription removes a subscription by ID.
func (s *Store) DeleteSubscription(id int64) error {
	_, err := s.db.Exec("DELETE FROM subscriptions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("store: delete subscription: %w", err)
	}
	return nil
}

// GetSubscription retrieves a single subscription by ID.
func (s *Store) GetSubscription(id int64) (*Subscription, error) {
	row := s.db.QueryRow(`
		SELECT id, platform, category, icon_key, email, plan_name, status,
			cost_amount, cost_currency, billing_cycle,
			token_limit, credit_limit, request_limit, limit_period, limit_note,
			next_renewal, started_at, trial_ends_at,
			notes, url, status_page_url,
			auto_tracked, account_id, created_at, updated_at
		FROM subscriptions WHERE id = ?
	`, id)

	sub, err := scanSubscription(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("store: get subscription: %w", err)
	}
	return sub, nil
}

// ListSubscriptions returns all subscriptions, optionally filtered.
func (s *Store) ListSubscriptions(status, category string) ([]*Subscription, error) {
	query := `
		SELECT id, platform, category, icon_key, email, plan_name, status,
			cost_amount, cost_currency, billing_cycle,
			token_limit, credit_limit, request_limit, limit_period, limit_note,
			next_renewal, started_at, trial_ends_at,
			notes, url, status_page_url,
			auto_tracked, account_id, created_at, updated_at
		FROM subscriptions WHERE 1=1
	`
	var args []interface{}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}
	query += " ORDER BY category, platform"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub, err := scanSubscriptionRow(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// FindSubscriptionByAccountID finds a subscription linked to an auto-tracked account.
func (s *Store) FindSubscriptionByAccountID(accountID int64) (*Subscription, error) {
	row := s.db.QueryRow(`
		SELECT id, platform, category, icon_key, email, plan_name, status,
			cost_amount, cost_currency, billing_cycle,
			token_limit, credit_limit, request_limit, limit_period, limit_note,
			next_renewal, started_at, trial_ends_at,
			notes, url, status_page_url,
			auto_tracked, account_id, created_at, updated_at
		FROM subscriptions WHERE auto_tracked = 1 AND account_id = ?
	`, accountID)

	sub, err := scanSubscription(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("store: find subscription by account: %w", err)
	}
	return sub, nil
}

// SubscriptionCount returns the total number of subscriptions.
func (s *Store) SubscriptionCount() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM subscriptions").Scan(&count)
	return count
}

// SubscriptionOverview computes spending and status summaries.
type OverviewStats struct {
	TotalMonthlySpend float64            `json:"totalMonthlySpend"`
	TotalAnnualSpend  float64            `json:"totalAnnualSpend"`
	ByCategory        map[string]CatStat `json:"byCategory"`
	ByStatus          map[string]int     `json:"byStatus"`
}

type CatStat struct {
	Count        int     `json:"count"`
	MonthlySpend float64 `json:"monthlySpend"`
}

func (s *Store) SubscriptionOverview() (*OverviewStats, error) {
	subs, err := s.ListSubscriptions("", "")
	if err != nil {
		return nil, err
	}

	stats := &OverviewStats{
		ByCategory: make(map[string]CatStat),
		ByStatus:   make(map[string]int),
	}

	for _, sub := range subs {
		monthly := toMonthly(sub.CostAmount, sub.BillingCycle)
		stats.TotalMonthlySpend += monthly
		stats.TotalAnnualSpend += monthly * 12

		cat := stats.ByCategory[sub.Category]
		cat.Count++
		cat.MonthlySpend += monthly
		stats.ByCategory[sub.Category] = cat

		stats.ByStatus[sub.Status]++
	}

	return stats, nil
}

// UpcomingRenewals returns subscriptions with upcoming renewals, sorted by date.
func (s *Store) UpcomingRenewals(limit int) ([]*Subscription, error) {
	if limit <= 0 {
		limit = 10
	}
	today := time.Now().Format("2006-01-02")

	rows, err := s.db.Query(`
		SELECT id, platform, category, icon_key, email, plan_name, status,
			cost_amount, cost_currency, billing_cycle,
			token_limit, credit_limit, request_limit, limit_period, limit_note,
			next_renewal, started_at, trial_ends_at,
			notes, url, status_page_url,
			auto_tracked, account_id, created_at, updated_at
		FROM subscriptions
		WHERE next_renewal != '' AND next_renewal >= ? AND status IN ('active', 'trial')
		ORDER BY next_renewal ASC
		LIMIT ?
	`, today, limit)
	if err != nil {
		return nil, fmt.Errorf("store: upcoming renewals: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub, err := scanSubscriptionRow(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// Insight represents a server-computed subscription analysis finding.
type Insight struct {
	Type     string `json:"type"`     // "unused", "renewal", "anomaly", "overlap", "savings", "trial"
	Severity string `json:"severity"` // "info", "warning", "critical"
	Icon     string `json:"icon"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Platform string `json:"platform,omitempty"`
	SubID    int64  `json:"subId,omitempty"`
}

// GenerateInsights analyzes subscriptions and returns actionable findings.
func (s *Store) GenerateInsights() ([]Insight, error) {
	subs, err := s.ListSubscriptions("", "")
	if err != nil {
		return nil, err
	}

	var insights []Insight
	now := time.Now()
	budget := s.GetConfigFloat("budget_monthly", 0)
	currency := s.GetConfig("currency")
	if currency == "" {
		currency = "USD"
	}

	// Category counter
	catCounts := make(map[string]int)
	totalMonthly := 0.0

	for _, sub := range subs {
		if sub.Status != "active" && sub.Status != "trial" {
			continue
		}

		monthly := toMonthly(sub.CostAmount, sub.BillingCycle)
		totalMonthly += monthly
		catCounts[sub.Category]++

		// Rule 1: Imminent renewal (within 3 days) → critical
		if sub.NextRenewal != "" {
			if t, err := time.Parse("2006-01-02", sub.NextRenewal); err == nil {
				days := int(t.Sub(now).Hours() / 24)
				if days >= 0 && days <= 3 {
					insights = append(insights, Insight{
						Type:     "renewal",
						Severity: "critical",
						Icon:     "⏰",
						Title:    "Imminent Renewal",
						Message:  fmt.Sprintf("%s renews in %d day(s) (%s %.2f)", sub.Platform, days, currency, sub.CostAmount),
						Platform: sub.Platform,
						SubID:    sub.ID,
					})
				}
			}
		}

		// Rule 2: Trial expiring within 5 days → warning
		if sub.Status == "trial" && sub.TrialEndsAt != "" {
			if t, err := time.Parse("2006-01-02", sub.TrialEndsAt); err == nil {
				days := int(t.Sub(now).Hours() / 24)
				if days >= 0 && days <= 5 {
					insights = append(insights, Insight{
						Type:     "trial",
						Severity: "warning",
						Icon:     "⏳",
						Title:    "Trial Expiring",
						Message:  fmt.Sprintf("%s trial ends in %d day(s) — cancel or it converts to paid", sub.Platform, days),
						Platform: sub.Platform,
						SubID:    sub.ID,
					})
				}
			}
		}

		// Rule 4: Unused subscription — auto-tracked sub with no recent snapshot
		if sub.AutoTracked && sub.AccountID > 0 {
			var lastSnap string
			s.db.QueryRow(
				`SELECT MAX(captured_at) FROM snapshots WHERE account_id = ?`, sub.AccountID,
			).Scan(&lastSnap)
			if lastSnap != "" {
				if t, err := time.Parse("2006-01-02 15:04:05", lastSnap); err == nil {
					daysSince := int(now.Sub(t).Hours() / 24)
					if daysSince > 30 {
						insights = append(insights, Insight{
							Type:     "unused",
							Severity: "warning",
							Icon:     "💤",
							Title:    "Possibly Unused",
							Message:  fmt.Sprintf("%s has not been used in %d days — consider cancelling", sub.Platform, daysSince),
							Platform: sub.Platform,
							SubID:    sub.ID,
						})
					}
				}
			}
		}
	}

	// Rule 3: Annual savings (deduplicated per-platform, separate loop)
	savingsSeen := make(map[string]bool)
	for _, sub := range subs {
		if sub.Status != "active" || sub.BillingCycle != "monthly" {
			continue
		}
		m := toMonthly(sub.CostAmount, sub.BillingCycle)
		if m < 10 || savingsSeen[sub.Platform] {
			continue
		}
		savingsSeen[sub.Platform] = true
		annualSaving := m * 12 * 0.17
		insights = append(insights, Insight{
			Type:     "annual_savings",
			Severity: "info",
			Icon:     "💰",
			Title:    "Annual Billing Saves Money",
			Message:  fmt.Sprintf("Switching %s to annual billing could save ~%s %.0f/year", sub.Platform, currency, annualSaving),
			Platform: sub.Platform,
			SubID:    sub.ID,
		})
	}

	// Rule 5: Spending anomaly — over 2× budget → critical
	if budget > 0 && totalMonthly > budget*2 {
		insights = append(insights, Insight{
			Type:     "anomaly",
			Severity: "critical",
			Icon:     "📈",
			Title:    "Spending Warning",
			Message:  fmt.Sprintf("Monthly spend (%s %.0f) is %.0f%% of budget (%s %.0f)", currency, totalMonthly, totalMonthly/budget*100, currency, budget),
		})
	}

	// Rule 6: Category overlap — 3+ active subs in same category → info
	for cat, count := range catCounts {
		if count >= 3 {
			insights = append(insights, Insight{
				Type:     "overlap",
				Severity: "info",
				Icon:     "🔄",
				Title:    "Category Overlap",
				Message:  fmt.Sprintf("%d active subscriptions in '%s' — consider consolidating", count, cat),
			})
		}
	}

	return insights, nil
}

// ── Helpers ──

func scanSubscription(row *sql.Row) (*Subscription, error) {
	var sub Subscription
	var autoTracked int
	err := row.Scan(
		&sub.ID, &sub.Platform, &sub.Category, &sub.IconKey,
		&sub.Email, &sub.PlanName, &sub.Status,
		&sub.CostAmount, &sub.CostCurrency, &sub.BillingCycle,
		&sub.TokenLimit, &sub.CreditLimit, &sub.RequestLimit,
		&sub.LimitPeriod, &sub.LimitNote,
		&sub.NextRenewal, &sub.StartedAt, &sub.TrialEndsAt,
		&sub.Notes, &sub.URL, &sub.StatusPageURL,
		&autoTracked, &sub.AccountID, &sub.CreatedAt, &sub.UpdatedAt,
	)
	sub.AutoTracked = autoTracked == 1
	return &sub, err
}

func scanSubscriptionRow(rows *sql.Rows) (*Subscription, error) {
	var sub Subscription
	var autoTracked int
	err := rows.Scan(
		&sub.ID, &sub.Platform, &sub.Category, &sub.IconKey,
		&sub.Email, &sub.PlanName, &sub.Status,
		&sub.CostAmount, &sub.CostCurrency, &sub.BillingCycle,
		&sub.TokenLimit, &sub.CreditLimit, &sub.RequestLimit,
		&sub.LimitPeriod, &sub.LimitNote,
		&sub.NextRenewal, &sub.StartedAt, &sub.TrialEndsAt,
		&sub.Notes, &sub.URL, &sub.StatusPageURL,
		&autoTracked, &sub.AccountID, &sub.CreatedAt, &sub.UpdatedAt,
	)
	sub.AutoTracked = autoTracked == 1
	return &sub, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ToMonthlyExported converts a cost amount to its monthly equivalent based on billing cycle.
// Exported for use by the CSV export handler.
func ToMonthlyExported(amount float64, cycle string) float64 {
	return toMonthly(amount, cycle)
}

func toMonthly(amount float64, cycle string) float64 {
	switch cycle {
	case "annual":
		return amount / 12
	case "lifetime":
		return 0 // one-time; don't include in monthly
	case "payg":
		return amount // user enters estimated monthly
	default: // monthly
		return amount
	}
}
