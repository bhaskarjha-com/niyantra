package costtrack

import (
	"math"
	"testing"
)

func TestBlendedPricePerToken(t *testing.T) {
	p := ModelPricing{
		ModelID:     "claude-sonnet-4.6",
		InputPer1M:  3.00,
		OutputPer1M: 15.00,
	}

	// blended = 0.4 * 3.00 + 0.6 * 15.00 = 1.20 + 9.00 = 10.20 per 1M
	// per token = 10.20 / 1_000_000 = 0.0000102
	got := p.BlendedPricePerToken()
	want := 10.20 / 1_000_000
	if math.Abs(got-want) > 1e-12 {
		t.Errorf("BlendedPricePerToken() = %v, want %v", got, want)
	}
}

func TestBlendedPricePerToken_ZeroPricing(t *testing.T) {
	p := ModelPricing{}
	got := p.BlendedPricePerToken()
	if got != 0 {
		t.Errorf("BlendedPricePerToken() = %v, want 0", got)
	}
}

func simpleAssigner(modelID string) string {
	switch modelID {
	case "claude-sonnet-4.6", "claude-opus-4.6", "gpt-4o":
		return "claude_gpt"
	case "gemini-3.1-pro":
		return "gemini_pro"
	case "gemini-2.5-flash":
		return "gemini_flash"
	default:
		return "claude_gpt"
	}
}

func testPricing() []ModelPricing {
	return []ModelPricing{
		{ModelID: "claude-sonnet-4.6", Provider: "anthropic", InputPer1M: 3.00, OutputPer1M: 15.00},
		{ModelID: "gpt-4o", Provider: "openai", InputPer1M: 2.50, OutputPer1M: 10.00},
		{ModelID: "gemini-3.1-pro", Provider: "google", InputPer1M: 2.00, OutputPer1M: 12.00},
		{ModelID: "gemini-2.5-flash", Provider: "google", InputPer1M: 0.30, OutputPer1M: 2.50},
	}
}

func TestEstimateGroupCost_ClaudeGPT(t *testing.T) {
	rate := GroupRate{
		GroupKey:   "claude_gpt",
		BurnRate:  0.10, // 10%/hr
		Remaining: 0.60, // 40% consumed
		HasData:   true,
	}
	ceiling := GroupCeiling{
		GroupKey:            "claude_gpt",
		DisplayName:        "Claude + GPT",
		TokensPerCycle:     5_000_000,
		CycleDurationHours: 5,
	}

	est := EstimateGroupCost(rate, ceiling, testPricing(), simpleAssigner)

	if !est.HasData {
		t.Fatal("expected HasData = true")
	}
	if est.GroupKey != "claude_gpt" {
		t.Errorf("GroupKey = %q, want %q", est.GroupKey, "claude_gpt")
	}

	// consumed = 0.40
	if math.Abs(est.ConsumedFrac-0.40) > 0.01 {
		t.Errorf("ConsumedFrac = %v, want ~0.40", est.ConsumedFrac)
	}

	// tokens = 0.40 * 5M = 2M
	if math.Abs(est.EstTokens-2_000_000) > 1000 {
		t.Errorf("EstTokens = %v, want ~2,000,000", est.EstTokens)
	}

	// Cost should be positive
	if est.EstCost <= 0 {
		t.Errorf("EstCost = %v, want > 0", est.EstCost)
	}

	// Hourly cost should be positive
	if est.CostPerHour <= 0 {
		t.Errorf("CostPerHour = %v, want > 0", est.CostPerHour)
	}

	// Labels should be formatted
	if est.CostLabel == "—" || est.CostLabel == "" {
		t.Errorf("CostLabel = %q, want formatted cost", est.CostLabel)
	}
	if est.HourlyLabel == "—" || est.HourlyLabel == "" {
		t.Errorf("HourlyLabel = %q, want formatted cost", est.HourlyLabel)
	}

	t.Logf("Claude+GPT: consumed=%.0f%%, tokens=%.0f, cost=%s, hourly=%s",
		est.ConsumedFrac*100, est.EstTokens, est.CostLabel, est.HourlyLabel)
}

func TestEstimateGroupCost_NoData(t *testing.T) {
	rate := GroupRate{
		GroupKey: "claude_gpt",
		HasData: false,
	}
	ceiling := GroupCeiling{
		TokensPerCycle: 5_000_000,
	}

	est := EstimateGroupCost(rate, ceiling, testPricing(), simpleAssigner)

	if est.HasData {
		t.Error("expected HasData = false")
	}
	if est.CostLabel != "—" {
		t.Errorf("CostLabel = %q, want %q", est.CostLabel, "—")
	}
}

func TestEstimateGroupCost_FullRemaining(t *testing.T) {
	rate := GroupRate{
		GroupKey:   "claude_gpt",
		BurnRate:  0.05,
		Remaining: 1.0, // nothing consumed
		HasData:   true,
	}
	ceiling := GroupCeiling{
		GroupKey:        "claude_gpt",
		DisplayName:    "Claude + GPT",
		TokensPerCycle: 5_000_000,
	}

	est := EstimateGroupCost(rate, ceiling, testPricing(), simpleAssigner)

	if est.ConsumedFrac != 0 {
		t.Errorf("ConsumedFrac = %v, want 0", est.ConsumedFrac)
	}
	if est.EstCost != 0 {
		t.Errorf("EstCost = %v, want 0", est.EstCost)
	}
	// Hourly cost should still be positive since burn rate exists
	if est.CostPerHour <= 0 {
		t.Errorf("CostPerHour = %v, want > 0", est.CostPerHour)
	}
}

func TestEstimateGroupCost_GeminiFlash(t *testing.T) {
	rate := GroupRate{
		GroupKey:   "gemini_flash",
		BurnRate:  0.20,
		Remaining: 0.30, // 70% consumed
		HasData:   true,
	}
	ceiling := GroupCeiling{
		GroupKey:            "gemini_flash",
		DisplayName:        "Gemini Flash",
		TokensPerCycle:     10_000_000,
		CycleDurationHours: 5,
	}

	est := EstimateGroupCost(rate, ceiling, testPricing(), simpleAssigner)

	// Flash is cheap — cost should be lower than Claude+GPT for same consumption
	if est.EstCost <= 0 {
		t.Errorf("EstCost = %v, want > 0", est.EstCost)
	}

	t.Logf("Gemini Flash: consumed=%.0f%%, tokens=%.0f, cost=%s, hourly=%s",
		est.ConsumedFrac*100, est.EstTokens, est.CostLabel, est.HourlyLabel)
}

func TestEstimateAccountCost(t *testing.T) {
	rates := []GroupRate{
		{GroupKey: "claude_gpt", BurnRate: 0.10, Remaining: 0.60, HasData: true},
		{GroupKey: "gemini_pro", BurnRate: 0.05, Remaining: 0.80, HasData: true},
		{GroupKey: "gemini_flash", BurnRate: 0.20, Remaining: 0.30, HasData: true},
	}

	est := EstimateAccountCost(1, "user@example.com", rates, DefaultQuotaCeilings(), testPricing(), simpleAssigner)

	if est.AccountID != 1 {
		t.Errorf("AccountID = %d, want 1", est.AccountID)
	}
	if est.TotalCost <= 0 {
		t.Errorf("TotalCost = %v, want > 0", est.TotalCost)
	}
	if len(est.Groups) != 3 {
		t.Errorf("len(Groups) = %d, want 3", len(est.Groups))
	}
	if est.TotalLabel == "" || est.TotalLabel == "$0.00" {
		t.Errorf("TotalLabel = %q, want non-zero cost", est.TotalLabel)
	}

	t.Logf("Account total: %s (%d groups)", est.TotalLabel, len(est.Groups))
	for _, g := range est.Groups {
		t.Logf("  %s: %s (hourly: %s)", g.DisplayName, g.CostLabel, g.HourlyLabel)
	}
}

func TestEstimateAccountCost_EmptyRates(t *testing.T) {
	est := EstimateAccountCost(1, "user@example.com", nil, DefaultQuotaCeilings(), testPricing(), simpleAssigner)

	if est.TotalCost != 0 {
		t.Errorf("TotalCost = %v, want 0", est.TotalCost)
	}
	if len(est.Groups) != 0 {
		t.Errorf("len(Groups) = %d, want 0", len(est.Groups))
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "$0.00"},
		{-1, "$0.00"},
		{0.005, "<$0.01"},
		{0.01, "$0.01"},
		{1.234, "$1.23"},
		{12.5, "$12.50"},
		{100.999, "$101.00"},
	}

	for _, tt := range tests {
		got := FormatCost(tt.input)
		if got != tt.want {
			t.Errorf("FormatCost(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDefaultQuotaCeilings(t *testing.T) {
	c := DefaultQuotaCeilings()
	if len(c) != 3 {
		t.Fatalf("DefaultQuotaCeilings() has %d groups, want 3", len(c))
	}

	for _, key := range []string{"claude_gpt", "gemini_pro", "gemini_flash"} {
		g, ok := c[key]
		if !ok {
			t.Errorf("missing group %q", key)
			continue
		}
		if g.TokensPerCycle <= 0 {
			t.Errorf("%s: TokensPerCycle = %v, want > 0", key, g.TokensPerCycle)
		}
		if g.CycleDurationHours <= 0 {
			t.Errorf("%s: CycleDurationHours = %v, want > 0", key, g.CycleDurationHours)
		}
	}
}

func TestParseCeilings_Empty(t *testing.T) {
	c, err := ParseCeilings("")
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 3 {
		t.Errorf("ParseCeilings('') returned %d ceilings, want 3 (defaults)", len(c))
	}
}

func TestParseCeilings_Custom(t *testing.T) {
	raw := `{"claude_gpt":{"groupKey":"claude_gpt","displayName":"Claude + GPT","tokensPerCycle":8000000,"cycleDurationHours":5}}`
	c, err := ParseCeilings(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c["claude_gpt"].TokensPerCycle != 8_000_000 {
		t.Errorf("claude_gpt.TokensPerCycle = %v, want 8000000", c["claude_gpt"].TokensPerCycle)
	}
}

func TestParseCeilings_Invalid(t *testing.T) {
	_, err := ParseCeilings("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMarshalCeilings(t *testing.T) {
	c := DefaultQuotaCeilings()
	raw, err := MarshalCeilings(c)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		t.Error("MarshalCeilings returned empty string")
	}

	// Round-trip
	parsed, err := ParseCeilings(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != len(c) {
		t.Errorf("round-trip: got %d ceilings, want %d", len(parsed), len(c))
	}
}
