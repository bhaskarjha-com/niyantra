package client

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Quota group constants.
const (
	GroupClaudeGPT   = "claude_gpt"
	GroupGeminiPro   = "gemini_pro"
	GroupGeminiFlash = "gemini_flash"
)

// GroupOrder defines the canonical display order.
var GroupOrder = []string{GroupClaudeGPT, GroupGeminiPro, GroupGeminiFlash}

// GroupDisplayNames maps group keys to human-readable names.
var GroupDisplayNames = map[string]string{
	GroupClaudeGPT:   "Claude + GPT",
	GroupGeminiPro:   "Gemini Pro",
	GroupGeminiFlash: "Gemini Flash",
}

// GroupColors maps group keys to display colors.
var GroupColors = map[string]string{
	GroupClaudeGPT:   "#D97757",
	GroupGeminiPro:   "#10B981",
	GroupGeminiFlash: "#3B82F6",
}

// --- API response types (from Antigravity Connect RPC) ---

// ModelOrAlias is the model identifier in the API response.
type ModelOrAlias struct {
	Model string `json:"model"`
}

// QuotaInfo contains remaining quota and reset time for a model.
// Note: remainingFraction uses protobuf semantics — when the field is
// omitted from JSON, it means 0.0 (exhausted), because protobuf
// serializers omit fields equal to their default value (0 for floats).
type QuotaInfo struct {
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

// ModelConfig is a single model's configuration from the API.
type ModelConfig struct {
	Label        string        `json:"label"`
	ModelOrAlias *ModelOrAlias `json:"modelOrAlias,omitempty"`
	QuotaInfo    *QuotaInfo    `json:"quotaInfo,omitempty"`
}

// PlanInfo contains subscription plan details.
type PlanInfo struct {
	PlanName             string `json:"planName"`
	MonthlyPromptCredits int    `json:"monthlyPromptCredits"`
}

// PlanStatus contains plan info with available credits.
type PlanStatus struct {
	PlanInfo               *PlanInfo `json:"planInfo,omitempty"`
	AvailablePromptCredits float64   `json:"availablePromptCredits"`
}

// AICredit represents a single AI credit pool (e.g., Google One AI credits).
type AICredit struct {
	CreditType      string  `json:"creditType"`      // e.g. "GOOGLE_ONE_AI"
	CreditAmount    float64 `json:"creditAmount"`    // parsed from API string
	MinimumForUsage float64 `json:"minimumForUsage"` // min balance to use credits
}

// rawCredit matches the API response shape (amounts are strings).
type rawCredit struct {
	CreditType                  string `json:"creditType"`
	CreditAmount                string `json:"creditAmount"`
	MinimumCreditAmountForUsage string `json:"minimumCreditAmountForUsage"`
}

// UserTier contains the user's subscription tier and AI credit balance.
type UserTier struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	AvailableCredits []rawCredit `json:"availableCredits"`
}

// CascadeModelConfigData wraps the model configs array.
type CascadeModelConfigData struct {
	ClientModelConfigs []ModelConfig `json:"clientModelConfigs"`
}

// UserStatus is the user status from the API.
type UserStatus struct {
	Name                   string                  `json:"name"`
	Email                  string                  `json:"email"`
	PlanStatus             *PlanStatus             `json:"planStatus,omitempty"`
	CascadeModelConfigData *CascadeModelConfigData `json:"cascadeModelConfigData,omitempty"`
	UserTier               *UserTier               `json:"userTier,omitempty"`
}

// UserStatusResponse is the full API response.
type UserStatusResponse struct {
	UserStatus      *UserStatus `json:"userStatus"`
	Message         string      `json:"message,omitempty"`
	Code            string      `json:"code,omitempty"`
	OriginalRawJSON string      `json:"-"`
}

// --- Normalized types for storage ---

// ModelQuota is a normalized model quota for storage.
type ModelQuota struct {
	ModelID           string        `json:"modelId"`
	Label             string        `json:"label"`
	RemainingFraction float64       `json:"remainingFraction"`
	RemainingPercent  float64       `json:"remainingPercent"`
	IsExhausted       bool          `json:"isExhausted"`
	ResetTime         *time.Time    `json:"resetTime,omitempty"`
	TimeUntilReset    time.Duration `json:"-"`
}

// Snapshot is a point-in-time capture of Antigravity quotas.
type Snapshot struct {
	ID             int64
	AccountID      int64
	CapturedAt     time.Time
	Email          string
	PlanName       string
	PromptCredits  float64
	MonthlyCredits int
	Models         []ModelQuota
	AICredits      []AICredit
	RawJSON        string
	CaptureMethod  string // "manual" or "auto"
	CaptureSource  string // "cli", "ui", "watch", "parser", "import", "mcp"
	SourceID       string // "antigravity", "claude_code", "codex"
}

// GroupedQuota represents one logical quota group (e.g., Claude+GPT).
// N6: RemainingPercent is always RemainingFraction * 100.
// Both fields exist for convenience — fraction for internal math, percent for display.
type GroupedQuota struct {
	GroupKey          string
	DisplayName       string
	RemainingFraction float64 // 0.0–1.0 scale (for internal calculations)
	RemainingPercent  float64 // 0–100 scale (for display/JSON output)
	IsExhausted       bool
	ResetTime         *time.Time
	TimeUntilReset    time.Duration
	Color             string
}

// --- Conversion ---

// ToSnapshot converts an API response to a Snapshot.
func (r *UserStatusResponse) ToSnapshot(capturedAt time.Time) *Snapshot {
	snap := &Snapshot{CapturedAt: capturedAt}

	if r.UserStatus == nil {
		return snap
	}

	snap.Email = r.UserStatus.Email

	if r.UserStatus.PlanStatus != nil {
		snap.PromptCredits = r.UserStatus.PlanStatus.AvailablePromptCredits
		if r.UserStatus.PlanStatus.PlanInfo != nil {
			snap.PlanName = r.UserStatus.PlanStatus.PlanInfo.PlanName
			snap.MonthlyCredits = r.UserStatus.PlanStatus.PlanInfo.MonthlyPromptCredits
		}
	}

	if r.UserStatus.CascadeModelConfigData != nil {
		now := time.Now()
		for _, cfg := range r.UserStatus.CascadeModelConfigData.ClientModelConfigs {
			if cfg.QuotaInfo == nil {
				continue
			}

			modelID := ""
			if cfg.ModelOrAlias != nil {
				modelID = cfg.ModelOrAlias.Model
			}

			mq := ModelQuota{
				ModelID:           modelID,
				Label:             cleanLabel(cfg.Label),
				RemainingFraction: cfg.QuotaInfo.RemainingFraction,
				RemainingPercent:  cfg.QuotaInfo.RemainingFraction * 100,
				IsExhausted:       cfg.QuotaInfo.RemainingFraction == 0,
			}

			if cfg.QuotaInfo.ResetTime != "" {
				if t, err := time.Parse(time.RFC3339, cfg.QuotaInfo.ResetTime); err == nil {
					mq.ResetTime = &t
					mq.TimeUntilReset = t.Sub(now)
					if mq.TimeUntilReset < 0 {
						mq.TimeUntilReset = 0
					}
				}
			}

			snap.Models = append(snap.Models, mq)
		}
	}

	// Parse AI credits from userTier
	if r.UserStatus.UserTier != nil {
		for _, rc := range r.UserStatus.UserTier.AvailableCredits {
			if rc.CreditType == "" {
				continue
			}
			amount, _ := strconv.ParseFloat(rc.CreditAmount, 64)
			minUsage, _ := strconv.ParseFloat(rc.MinimumCreditAmountForUsage, 64)
			snap.AICredits = append(snap.AICredits, AICredit{
				CreditType:      rc.CreditType,
				CreditAmount:    amount,
				MinimumForUsage: minUsage,
			})
		}
	}

	if r.OriginalRawJSON != "" {
		snap.RawJSON = r.OriginalRawJSON
	} else if raw, err := json.Marshal(r); err == nil {
		snap.RawJSON = string(raw)
	}

	return snap
}

// ActiveModelIDs returns sorted model IDs from the response.
func (r *UserStatusResponse) ActiveModelIDs() []string {
	if r.UserStatus == nil || r.UserStatus.CascadeModelConfigData == nil {
		return nil
	}
	var ids []string
	for _, cfg := range r.UserStatus.CascadeModelConfigData.ClientModelConfigs {
		if cfg.QuotaInfo != nil && cfg.ModelOrAlias != nil {
			ids = append(ids, cfg.ModelOrAlias.Model)
		}
	}
	sort.Strings(ids)
	return ids
}

// --- Model grouping ---

// GroupForModel determines which quota group a model belongs to.
func GroupForModel(modelID, label string) string {
	text := strings.ToLower(modelID + " " + label)

	switch {
	case strings.Contains(text, "gemini") && strings.Contains(text, "flash"):
		return GroupGeminiFlash
	case strings.Contains(text, "gemini"):
		return GroupGeminiPro
	default:
		return GroupClaudeGPT
	}
}

// GroupModels groups model quotas into logical quota groups.
func GroupModels(models []ModelQuota) []GroupedQuota {
	type acc struct {
		sum           float64
		count         int
		anyExhausted  bool
		earliestReset *time.Time
	}

	byGroup := map[string]*acc{}
	for _, key := range GroupOrder {
		byGroup[key] = &acc{}
	}

	for _, m := range models {
		key := GroupForModel(m.ModelID, m.Label)
		a := byGroup[key]
		if a == nil {
			a = &acc{}
			byGroup[key] = a
		}
		a.sum += m.RemainingFraction
		a.count++
		a.anyExhausted = a.anyExhausted || m.IsExhausted || m.RemainingFraction <= 0

		if m.ResetTime != nil {
			if a.earliestReset == nil || m.ResetTime.Before(*a.earliestReset) {
				t := *m.ResetTime
				a.earliestReset = &t
			}
		}
	}

	now := time.Now().UTC()
	groups := make([]GroupedQuota, 0, len(GroupOrder))

	for _, key := range GroupOrder {
		a := byGroup[key]
		remaining := 1.0
		if a != nil && a.count > 0 {
			remaining = a.sum / float64(a.count)
		}
		if remaining < 0 {
			remaining = 0
		}
		if remaining > 1 {
			remaining = 1
		}

		g := GroupedQuota{
			GroupKey:          key,
			DisplayName:       GroupDisplayNames[key],
			RemainingFraction: remaining,
			RemainingPercent:  remaining * 100,
			IsExhausted:       a != nil && a.count > 0 && remaining <= 0,
			Color:             GroupColors[key],
		}

		if a != nil && a.earliestReset != nil {
			g.ResetTime = a.earliestReset
			d := a.earliestReset.Sub(now)
			if d < 0 {
				d = 0
			}
			g.TimeUntilReset = d
		}

		groups = append(groups, g)
	}

	return groups
}

// cleanLabel removes redundant suffixes from model labels.
func cleanLabel(label string) string {
	label = strings.TrimSuffix(label, " (Thinking)")
	label = strings.TrimSuffix(label, "(Thinking)")
	return strings.TrimSpace(label)
}
