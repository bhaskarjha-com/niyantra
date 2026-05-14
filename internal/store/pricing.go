package store

import (
	"encoding/json"
	"fmt"
)

// ModelPrice holds per-model pricing in $/1M tokens.
type ModelPrice struct {
	ModelID      string  `json:"modelId"`
	DisplayName  string  `json:"displayName"`
	Provider     string  `json:"provider"`
	InputPer1M   float64 `json:"inputPer1M"`
	OutputPer1M  float64 `json:"outputPer1M"`
	CachePer1M   float64 `json:"cachePer1M"`
}

// DefaultModelPricing returns the pre-filled pricing defaults (May 2026 market rates).
func DefaultModelPricing() []ModelPrice {
	return []ModelPrice{
		{ModelID: "claude-opus-4.6", DisplayName: "Claude Opus 4.6", Provider: "anthropic", InputPer1M: 5.00, OutputPer1M: 25.00, CachePer1M: 0.50},
		{ModelID: "claude-sonnet-4.6", DisplayName: "Claude Sonnet 4.6", Provider: "anthropic", InputPer1M: 3.00, OutputPer1M: 15.00, CachePer1M: 0.30},
		{ModelID: "claude-haiku-4.5", DisplayName: "Claude Haiku 4.5", Provider: "anthropic", InputPer1M: 1.00, OutputPer1M: 5.00, CachePer1M: 0.10},
		{ModelID: "gpt-4o", DisplayName: "GPT-4o", Provider: "openai", InputPer1M: 2.50, OutputPer1M: 10.00, CachePer1M: 1.25},
		{ModelID: "gemini-3.1-pro", DisplayName: "Gemini 3.1 Pro", Provider: "google", InputPer1M: 2.00, OutputPer1M: 12.00, CachePer1M: 0.50},
		{ModelID: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash", Provider: "google", InputPer1M: 0.30, OutputPer1M: 2.50, CachePer1M: 0.075},
	}
}

// GetModelPricing returns the configured model pricing. If none is stored,
// it seeds the defaults and returns them.
func (s *Store) GetModelPricing() ([]ModelPrice, error) {
	raw := s.GetConfig("model_pricing")
	if raw == "" {
		// Seed defaults on first access (lazy init)
		defaults := DefaultModelPricing()
		if err := s.SetModelPricing(defaults); err != nil {
			return nil, fmt.Errorf("store: seeding model pricing: %w", err)
		}
		return defaults, nil
	}

	var prices []ModelPrice
	if err := json.Unmarshal([]byte(raw), &prices); err != nil {
		return nil, fmt.Errorf("store: parsing model pricing: %w", err)
	}
	return prices, nil
}

// SetModelPricing persists the model pricing array as JSON in the config table.
func (s *Store) SetModelPricing(prices []ModelPrice) error {
	data, err := json.Marshal(prices)
	if err != nil {
		return fmt.Errorf("store: marshalling model pricing: %w", err)
	}

	// Upsert into config table — the key may not exist yet in older databases
	_, err = s.db.Exec(`
		INSERT INTO config (key, value, value_type, category, label, description, updated_at)
		VALUES ('model_pricing', ?, 'json', 'pricing', 'Model Pricing', 'Per-model token pricing ($/1M tokens)', datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')
	`, string(data))
	if err != nil {
		return fmt.Errorf("store: saving model pricing: %w", err)
	}
	return nil
}

// GetModelPrice returns the pricing for a specific model by ID, or nil if not found.
// Uses exact match first, then falls back to prefix matching to handle
// version suffix differences (e.g. "claude-sonnet-4" → "claude-sonnet-4.6").
func (s *Store) GetModelPrice(modelID string) *ModelPrice {
	prices, err := s.GetModelPricing()
	if err != nil {
		return nil
	}

	// 1. Exact match
	for i := range prices {
		if prices[i].ModelID == modelID {
			return &prices[i]
		}
	}

	// 2. Prefix match: find the pricing entry whose ModelID starts with
	//    the queried modelID (or vice versa). Longest match wins.
	//    This handles "claude-sonnet-4" matching "claude-sonnet-4.6".
	var bestMatch *ModelPrice
	bestLen := 0
	for i := range prices {
		pid := prices[i].ModelID
		if len(pid) > len(modelID) {
			// pricing ID is longer: check if it starts with the query
			if len(modelID) > bestLen && startsWith(pid, modelID) {
				bestMatch = &prices[i]
				bestLen = len(modelID)
			}
		} else {
			// query is longer: check if query starts with the pricing ID
			if len(pid) > bestLen && startsWith(modelID, pid) {
				bestMatch = &prices[i]
				bestLen = len(pid)
			}
		}
	}
	return bestMatch
}

// startsWith checks if s starts with prefix.
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// GetQuotaCeilings returns the configured quota ceilings JSON string.
// Returns empty string if not configured (caller should use defaults).
func (s *Store) GetQuotaCeilings() string {
	return s.GetConfig("quota_ceilings")
}

// SetQuotaCeilings persists the quota ceilings JSON string in the config table.
func (s *Store) SetQuotaCeilings(raw string) error {
	_, err := s.db.Exec(`
		INSERT INTO config (key, value, value_type, category, label, description, updated_at)
		VALUES ('quota_ceilings', ?, 'json', 'pricing', 'Quota Ceilings', 'Assumed tokens per cycle per quota group (for cost estimation)', datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')
	`, raw)
	if err != nil {
		return fmt.Errorf("store: saving quota ceilings: %w", err)
	}
	return nil
}
