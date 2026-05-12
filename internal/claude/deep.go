package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ── Types ───────────────────────────────────────────────────────

// TokenUsage represents per-turn token usage from a Claude Code session.
type TokenUsage struct {
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	CacheReadTokens   int64  `json:"cache_read_input_tokens"`
	CacheCreateTokens int64  `json:"cache_creation_input_tokens"`
	Model             string `json:"model"`
	Timestamp         string `json:"timestamp"` // ISO 8601
	SessionID         string `json:"sessionId"`
}

// ModelUsage aggregates token usage for a single model.
type ModelUsage struct {
	Model        string  `json:"model"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CacheRead    int64   `json:"cacheRead"`
	CacheCreate  int64   `json:"cacheCreate"`
	CostUSD      float64 `json:"costUSD"`
	Turns        int     `json:"turns"`
}

// DailyUsage represents aggregated token/cost data for a single day.
type DailyUsage struct {
	Date         string                `json:"date"` // YYYY-MM-DD
	InputTokens  int64                 `json:"totalInput"`
	OutputTokens int64                 `json:"totalOutput"`
	CacheRead    int64                 `json:"totalCacheRead"`
	CacheCreate  int64                 `json:"totalCacheCreate"`
	CostUSD      float64               `json:"totalCost"`
	SessionCount int                   `json:"sessionCount"`
	ByModel      map[string]ModelUsage `json:"byModel"`
}

// UsageSummary is the full response returned by the deep tracking API.
type UsageSummary struct {
	Days          []DailyUsage `json:"days"`
	TotalCost     float64      `json:"totalCost"`
	TotalInput    int64        `json:"totalInput"`
	TotalOutput   int64        `json:"totalOutput"`
	TotalTokens   int64        `json:"totalTokens"`
	TotalSessions int          `json:"totalSessions"`
	CacheHitRate  float64      `json:"cacheHitRate"` // 0.0 - 1.0
	TopModel      string       `json:"topModel"`
}

// ModelPriceFunc is a callback to look up $/1M token pricing.
// Returns (inputPer1M, outputPer1M, cachePer1M, found).
type ModelPriceFunc func(modelID string) (float64, float64, float64, bool)

// ── JSONL Record Parsing ────────────────────────────────────────

// jsonlRecord is the minimal subset of a JSONL line we parse.
type jsonlRecord struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	Message   *messagePayload `json:"message,omitempty"`
}

type messagePayload struct {
	Model string       `json:"model"`
	Usage *usageObject `json:"usage,omitempty"`
}

type usageObject struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CacheRead    int64 `json:"cache_read_input_tokens"`
	CacheCreate  int64 `json:"cache_creation_input_tokens"`
}

// ── Core Functions ──────────────────────────────────────────────

// ClaudeDir returns the path to ~/.claude/
func ClaudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// ProjectsDir returns the path to ~/.claude/projects/
func ProjectsDir() string {
	d := ClaudeDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "projects")
}

// FindSessionFiles discovers all JSONL session files across all projects.
func FindSessionFiles() ([]string, error) {
	projDir := ProjectsDir()
	if projDir == "" {
		return nil, nil
	}

	info, err := os.Stat(projDir)
	if err != nil || !info.IsDir() {
		return nil, nil // No projects dir — graceful degradation
	}

	var files []string
	err = filepath.Walk(projDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ParseSessionFile reads a single JSONL file and extracts token usage
// from assistant turns. Returns all usage records found.
func ParseSessionFile(path string) ([]TokenUsage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	var usages []TokenUsage
	scanner := bufio.NewScanner(f)
	// Increase buffer for large JSONL lines (tool outputs can be huge)
	scanner.Buffer(make([]byte, 0, 256*1024), 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec jsonlRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // Skip malformed lines
		}

		// Only process assistant turns with usage data
		if rec.Type != "assistant" {
			continue
		}
		if rec.Message == nil || rec.Message.Usage == nil {
			continue
		}

		u := rec.Message.Usage
		// Skip records with zero tokens (thinking-only turns, etc.)
		if u.InputTokens == 0 && u.OutputTokens == 0 {
			continue
		}

		usages = append(usages, TokenUsage{
			InputTokens:       u.InputTokens,
			OutputTokens:      u.OutputTokens,
			CacheReadTokens:   u.CacheRead,
			CacheCreateTokens: u.CacheCreate,
			Model:             rec.Message.Model,
			Timestamp:         rec.Timestamp,
			SessionID:         rec.SessionID,
		})
	}

	return usages, scanner.Err()
}

// AggregateUsage parses all session files and returns a summary
// for the last N days. Uses priceFn to compute cost estimates.
func AggregateUsage(days int, priceFn ModelPriceFunc) (*UsageSummary, error) {
	if days <= 0 {
		days = 30
	}

	files, err := FindSessionFiles()
	if err != nil {
		return nil, fmt.Errorf("find session files: %w", err)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	cutoffStr := cutoff.Format("2006-01-02")

	// Collect all usage records across all sessions
	dayMap := make(map[string]*DailyUsage)
	sessionSet := make(map[string]bool)
	modelTotals := make(map[string]int64) // model → total tokens

	for _, path := range files {
		usages, err := ParseSessionFile(path)
		if err != nil {
			continue // Skip broken files
		}

		for _, u := range usages {
			// Parse timestamp to get date
			dateStr := extractDate(u.Timestamp)
			if dateStr == "" || dateStr < cutoffStr {
				continue // Before cutoff
			}

			// Get or create daily bucket
			day, ok := dayMap[dateStr]
			if !ok {
				day = &DailyUsage{
					Date:    dateStr,
					ByModel: make(map[string]ModelUsage),
				}
				dayMap[dateStr] = day
			}

			day.InputTokens += u.InputTokens
			day.OutputTokens += u.OutputTokens
			day.CacheRead += u.CacheReadTokens
			day.CacheCreate += u.CacheCreateTokens
			sessionSet[u.SessionID] = true

			// Per-model aggregation
			modelKey := normalizeModel(u.Model)
			mu := day.ByModel[modelKey]
			mu.Model = modelKey
			mu.InputTokens += u.InputTokens
			mu.OutputTokens += u.OutputTokens
			mu.CacheRead += u.CacheReadTokens
			mu.CacheCreate += u.CacheCreateTokens
			mu.Turns++

			// Cost calculation via price function
			if priceFn != nil {
				inPrice, outPrice, cachePrice, found := priceFn(modelKey)
				if found {
					mu.CostUSD += float64(u.InputTokens) / 1_000_000 * inPrice
					mu.CostUSD += float64(u.OutputTokens) / 1_000_000 * outPrice
					mu.CostUSD += float64(u.CacheReadTokens) / 1_000_000 * cachePrice
				}
			}

			day.ByModel[modelKey] = mu
			modelTotals[modelKey] += u.InputTokens + u.OutputTokens
		}
	}

	// Compute per-day costs and session counts
	var result []DailyUsage
	var totalCost float64
	var totalInput, totalOutput int64
	var totalCacheRead, totalCacheCreate int64

	for _, day := range dayMap {
		for _, mu := range day.ByModel {
			day.CostUSD += mu.CostUSD
		}
		// Count unique sessions per day (approximate: count sessions seen)
		day.SessionCount = len(day.ByModel) // rough proxy

		totalCost += day.CostUSD
		totalInput += day.InputTokens
		totalOutput += day.OutputTokens
		totalCacheRead += day.CacheRead
		totalCacheCreate += day.CacheCreate

		result = append(result, *day)
	}

	// Sort by date ascending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	// Compute session count per day more accurately
	// Re-pass: count unique sessions per day
	daySessionMap := make(map[string]map[string]bool)
	for _, path := range files {
		usages, _ := ParseSessionFile(path)
		for _, u := range usages {
			dateStr := extractDate(u.Timestamp)
			if dateStr == "" || dateStr < cutoffStr {
				continue
			}
			if daySessionMap[dateStr] == nil {
				daySessionMap[dateStr] = make(map[string]bool)
			}
			daySessionMap[dateStr][u.SessionID] = true
		}
	}
	for i := range result {
		if sessions, ok := daySessionMap[result[i].Date]; ok {
			result[i].SessionCount = len(sessions)
		}
	}

	// Find top model
	topModel := ""
	var topTokens int64
	for model, tokens := range modelTotals {
		if tokens > topTokens {
			topModel = model
			topTokens = tokens
		}
	}

	// Cache hit rate
	totalCacheOps := totalCacheRead + totalCacheCreate
	var cacheHitRate float64
	if totalCacheOps > 0 {
		cacheHitRate = float64(totalCacheRead) / float64(totalCacheOps)
	}

	return &UsageSummary{
		Days:          result,
		TotalCost:     totalCost,
		TotalInput:    totalInput,
		TotalOutput:   totalOutput,
		TotalTokens:   totalInput + totalOutput,
		TotalSessions: len(sessionSet),
		CacheHitRate:  cacheHitRate,
		TopModel:      topModel,
	}, nil
}

// ── Helpers ─────────────────────────────────────────────────────

// extractDate parses an ISO 8601 timestamp and returns the YYYY-MM-DD portion.
func extractDate(ts string) string {
	if len(ts) < 10 {
		return ""
	}
	// Fast path: just take first 10 chars if it looks like a date
	dateStr := ts[:10]
	if len(dateStr) == 10 && dateStr[4] == '-' && dateStr[7] == '-' {
		return dateStr
	}
	// Fallback: parse as time
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// normalizeModel extracts a clean model name from the full model identifier.
// e.g., "claude-sonnet-4-20250514" → "claude-sonnet-4"
func normalizeModel(model string) string {
	if model == "" {
		return "unknown"
	}
	// Remove date suffix if present (e.g., "-20250514")
	parts := strings.Split(model, "-")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if len(last) == 8 {
			// Check if it's all digits (date suffix)
			allDigits := true
			for _, c := range last {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return strings.Join(parts[:len(parts)-1], "-")
			}
		}
	}
	return model
}
