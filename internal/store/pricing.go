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
func (s *Store) GetModelPrice(modelID string) *ModelPrice {
	prices, err := s.GetModelPricing()
	if err != nil {
		return nil
	}
	for i := range prices {
		if prices[i].ModelID == modelID {
			return &prices[i]
		}
	}
	return nil
}
