package store

// ── Token Usage Persistence (F13: Token Usage Analytics) ─────────

// TokenUsageRow represents a single row in the token_usage table.
// Each row is a pre-computed daily aggregate for a (date, provider, model) tuple.
type TokenUsageRow struct {
	ID            int64   `json:"id"`
	Date          string  `json:"date"`          // YYYY-MM-DD
	Provider      string  `json:"provider"`      // claude, antigravity, codex, cursor, gemini
	Model         string  `json:"model"`         // model identifier (empty for provider-level aggregates)
	InputTokens   int64   `json:"inputTokens"`
	OutputTokens  int64   `json:"outputTokens"`
	CacheRead     int64   `json:"cacheRead"`
	CacheCreate   int64   `json:"cacheCreate"`
	EstimatedCost float64 `json:"estimatedCost"`
	TurnCount     int     `json:"turnCount"`
	SessionCount  int     `json:"sessionCount"`
	Source        string  `json:"source"` // "parsed" or "estimated"
}

// InsertTokenUsage upserts a daily token usage aggregate row.
// Uses UPSERT (INSERT OR REPLACE) to allow idempotent re-aggregation.
func (s *Store) InsertTokenUsage(row *TokenUsageRow) error {
	_, err := s.db.Exec(`
		INSERT INTO token_usage (date, provider, model, input_tokens, output_tokens,
			cache_read, cache_create, estimated_cost, turn_count, session_count, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, provider, model) DO UPDATE SET
			input_tokens   = excluded.input_tokens,
			output_tokens  = excluded.output_tokens,
			cache_read     = excluded.cache_read,
			cache_create   = excluded.cache_create,
			estimated_cost = excluded.estimated_cost,
			turn_count     = excluded.turn_count,
			session_count  = excluded.session_count,
			source         = excluded.source
	`, row.Date, row.Provider, row.Model, row.InputTokens, row.OutputTokens,
		row.CacheRead, row.CacheCreate, row.EstimatedCost, row.TurnCount,
		row.SessionCount, row.Source)
	return err
}

// QueryTokenUsage returns all token_usage rows matching the given filter criteria.
// cutoff is a YYYY-MM-DD date string; only rows on or after this date are returned.
// provider can be empty/"all" to return all providers, or a specific provider name.
func (s *Store) QueryTokenUsage(cutoff string, provider string) ([]TokenUsageRow, error) {
	var query string
	var args []interface{}

	if provider == "" || provider == "all" {
		query = `SELECT id, date, provider, model, input_tokens, output_tokens,
			cache_read, cache_create, estimated_cost, turn_count, session_count, source
			FROM token_usage WHERE date >= ? ORDER BY date ASC`
		args = []interface{}{cutoff}
	} else {
		query = `SELECT id, date, provider, model, input_tokens, output_tokens,
			cache_read, cache_create, estimated_cost, turn_count, session_count, source
			FROM token_usage WHERE date >= ? AND provider = ? ORDER BY date ASC`
		args = []interface{}{cutoff, provider}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TokenUsageRow
	for rows.Next() {
		var r TokenUsageRow
		if err := rows.Scan(&r.ID, &r.Date, &r.Provider, &r.Model,
			&r.InputTokens, &r.OutputTokens, &r.CacheRead, &r.CacheCreate,
			&r.EstimatedCost, &r.TurnCount, &r.SessionCount, &r.Source); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// DeleteTokenUsageBefore removes all token_usage rows older than the given date.
// Returns the number of rows deleted.
func (s *Store) DeleteTokenUsageBefore(cutoff string) (int64, error) {
	result, err := s.db.Exec("DELETE FROM token_usage WHERE date < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
