package store

// Preset is a pre-populated subscription template for quick onboarding.
type Preset struct {
	Platform      string  `json:"platform"`
	Category      string  `json:"category"`
	Cost          float64 `json:"costAmount"`
	Cycle         string  `json:"billingCycle"`
	TokenLimit    int64   `json:"tokenLimit,omitempty"`
	CreditLimit   int64   `json:"creditLimit,omitempty"`
	RequestLimit  int64   `json:"requestLimit,omitempty"`
	LimitPeriod   string  `json:"limitPeriod,omitempty"`
	Notes         string  `json:"notes"`
	URL           string  `json:"url"`
	StatusPageURL string  `json:"statusPageUrl"`
}

// Presets contains all 26 platform templates, grouped by category.
// These are starting points — users can customize all fields.
var Presets = []Preset{
	// ── AI Coding ──
	{
		Platform: "Antigravity Pro", Category: "coding",
		Cost: 15, Cycle: "monthly",
		Notes:         "5h sprint cycle quotas. AI Credits can be enabled in Settings > Models for overage. Multiple model pools: Claude+GPT, Gemini Pro, Gemini Flash.",
		URL:           "https://antigravity.google",
		StatusPageURL: "https://status.google.com",
	},
	{
		Platform: "Antigravity Pro+", Category: "coding",
		Cost: 60, Cycle: "monthly",
		Notes:         "Higher quotas than Pro. 5h sprint cycles. AI Credits for overage via Google One.",
		URL:           "https://antigravity.google",
		StatusPageURL: "https://status.google.com",
	},
	{
		Platform: "Cursor Pro", Category: "coding",
		Cost: 20, Cycle: "monthly",
		RequestLimit: 500, LimitPeriod: "monthly",
		Notes:         "500 fast premium requests/mo. Auto mode is unlimited on paid plans. Manual frontier model selection depletes credits faster.",
		URL:           "https://cursor.com/settings",
		StatusPageURL: "https://status.cursor.com",
	},
	{
		Platform: "Cursor Pro+", Category: "coding",
		Cost: 60, Cycle: "monthly",
		Notes:         "Higher limits than Pro. Unlimited slow requests.",
		URL:           "https://cursor.com/settings",
		StatusPageURL: "https://status.cursor.com",
	},
	{
		Platform: "Cursor Ultra", Category: "coding",
		Cost: 200, Cycle: "monthly",
		Notes:         "Highest tier. Unlimited fast requests.",
		URL:           "https://cursor.com/settings",
		StatusPageURL: "https://status.cursor.com",
	},
	{
		Platform: "GitHub Copilot Pro", Category: "coding",
		Cost: 10, Cycle: "monthly",
		RequestLimit: 300, LimitPeriod: "monthly",
		Notes:         "300 premium requests/mo. Overages: $0.04/request. Chat + completions included.",
		URL:           "https://github.com/settings/copilot",
		StatusPageURL: "https://www.githubstatus.com",
	},
	{
		Platform: "GitHub Copilot Pro+", Category: "coding",
		Cost: 39, Cycle: "monthly",
		RequestLimit: 1500, LimitPeriod: "monthly",
		Notes:         "1500 premium requests/mo. Agent mode + frontier models.",
		URL:           "https://github.com/settings/copilot",
		StatusPageURL: "https://www.githubstatus.com",
	},
	{
		Platform: "Codex (OpenAI)", Category: "coding",
		Cost: 200, Cycle: "monthly",
		Notes:         "Included with ChatGPT Pro subscription. Cloud-based agent for multi-file tasks.",
		URL:           "https://platform.openai.com",
		StatusPageURL: "https://status.openai.com",
	},
	{
		Platform: "Claude Code", Category: "coding",
		Cost: 20, Cycle: "monthly",
		LimitPeriod:   "rolling_5h",
		Notes:         "Included with Claude Pro. 5h rolling window (not calendar-based). Use /cost for session spend. /usage for limits. Max plans multiply allowance.",
		URL:           "https://claude.ai",
		StatusPageURL: "https://status.anthropic.com",
	},

	// ── AI Chat ──
	{
		Platform: "ChatGPT Plus", Category: "chat",
		Cost: 20, Cycle: "monthly",
		RequestLimit: 150, LimitPeriod: "rolling_3h",
		Notes:         "150 messages/3h on GPT-4o. Weekly limit on o3/o4-mini reasoning models. Deep Research limited uses.",
		URL:           "https://chat.openai.com",
		StatusPageURL: "https://status.openai.com",
	},
	{
		Platform: "ChatGPT Pro", Category: "chat",
		Cost: 200, Cycle: "monthly",
		Notes:         "Unlimited GPT-4o messages. Includes o3, deep research, Codex access. Effectively unlimited for most users.",
		URL:           "https://chat.openai.com",
		StatusPageURL: "https://status.openai.com",
	},
	{
		Platform: "Claude Pro", Category: "chat",
		Cost: 20, Cycle: "monthly",
		LimitPeriod:   "rolling_5h",
		Notes:         "5h rolling window. Includes Claude Code CLI. Shared quota with claude.ai web. Use /usage in CLI to check status.",
		URL:           "https://claude.ai/settings/billing",
		StatusPageURL: "https://status.anthropic.com",
	},
	{
		Platform: "Claude Max (5x)", Category: "chat",
		Cost: 100, Cycle: "monthly",
		LimitPeriod:   "rolling_5h",
		Notes:         "5x Pro usage allowance. Same 5h rolling window. Best for heavy Claude Code users.",
		URL:           "https://claude.ai/settings/billing",
		StatusPageURL: "https://status.anthropic.com",
	},
	{
		Platform: "Claude Max (20x)", Category: "chat",
		Cost: 200, Cycle: "monthly",
		LimitPeriod:   "rolling_5h",
		Notes:         "20x Pro usage allowance. For power users who hit limits daily.",
		URL:           "https://claude.ai/settings/billing",
		StatusPageURL: "https://status.anthropic.com",
	},
	{
		Platform: "Gemini Advanced", Category: "chat",
		Cost: 20, Cycle: "monthly",
		Notes:         "Part of Google One AI Premium. Includes 2TB storage. Access to Gemini 2.5 Pro.",
		URL:           "https://gemini.google.com",
		StatusPageURL: "https://status.google.com",
	},
	{
		Platform: "Perplexity Pro", Category: "chat",
		Cost: 20, Cycle: "monthly",
		Notes:         "Unlimited Pro Search. Access to Claude, GPT, and other models. Great for research.",
		URL:           "https://perplexity.ai/settings",
		StatusPageURL: "https://status.perplexity.ai",
	},
	{
		Platform: "Poe", Category: "chat",
		Cost: 20, Cycle: "monthly",
		CreditLimit:   1000000,
		Notes:         "Multi-model aggregator. 1M credits/mo. Access ChatGPT, Claude, Gemini, and custom bots.",
		URL:           "https://poe.com",
		StatusPageURL: "",
	},

	// ── AI API ──
	{
		Platform: "OpenAI API", Category: "api",
		Cost: 0, Cycle: "payg",
		Notes:         "Pay-per-token. Check usage at platform.openai.com/usage. Rate limits by tier (auto-upgrades with spend).",
		URL:           "https://platform.openai.com/usage",
		StatusPageURL: "https://status.openai.com",
	},
	{
		Platform: "Anthropic API", Category: "api",
		Cost: 0, Cycle: "payg",
		Notes:         "Pay-per-token. Prepaid credits. Check console.anthropic.com for usage.",
		URL:           "https://console.anthropic.com/settings/billing",
		StatusPageURL: "https://status.anthropic.com",
	},
	{
		Platform: "Google AI Studio", Category: "api",
		Cost: 0, Cycle: "payg",
		Notes:         "Free tier available. Pay-per-token beyond free limits.",
		URL:           "https://aistudio.google.com",
		StatusPageURL: "https://status.google.com",
	},
	{
		Platform: "OpenRouter", Category: "api",
		Cost: 0, Cycle: "payg",
		Notes:         "Multi-provider routing. Prepaid credits. Free models available. Great for cost optimization.",
		URL:           "https://openrouter.ai/activity",
		StatusPageURL: "",
	},

	// ── AI Image/Video ──
	{
		Platform: "Midjourney", Category: "image",
		Cost: 10, Cycle: "monthly",
		Notes:         "Basic: ~200 images/mo. Standard: ~900 images/mo. Pro: unlimited relaxed.",
		URL:           "https://midjourney.com/account",
		StatusPageURL: "",
	},
	{
		Platform: "Runway", Category: "image",
		Cost: 12, Cycle: "monthly",
		CreditLimit:   625,
		Notes:         "625 credits/mo (Standard). Video generation: ~25 credits per 5s clip.",
		URL:           "https://app.runway.ml",
		StatusPageURL: "",
	},

	// ── AI Audio ──
	{
		Platform: "ElevenLabs", Category: "audio",
		Cost: 5, Cycle: "monthly",
		Notes:         "Starter: 30,000 characters/mo. Voice cloning available on paid plans.",
		URL:           "https://elevenlabs.io/subscription",
		StatusPageURL: "",
	},
	{
		Platform: "Suno", Category: "audio",
		Cost: 8, Cycle: "monthly",
		CreditLimit:   500,
		Notes:         "500 credits/mo (~50 songs). Commercial use rights on paid plans.",
		URL:           "https://suno.com",
		StatusPageURL: "",
	},

	// ── AI Productivity ──
	{
		Platform: "Notion AI", Category: "productivity",
		Cost: 8, Cycle: "monthly",
		Notes:         "AI writing assistant, summaries, Q&A across workspace. Per-member pricing.",
		URL:           "https://notion.so",
		StatusPageURL: "https://status.notion.so",
	},
	{
		Platform: "Grammarly Premium", Category: "productivity",
		Cost: 12, Cycle: "monthly",
		Notes:         "AI writing suggestions, tone detection, plagiarism checker, full-sentence rewrites.",
		URL:           "https://grammarly.com",
		StatusPageURL: "",
	},
}
