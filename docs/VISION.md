# Vision: Niyantra

## What Is Niyantra?

Niyantra (Sanskrit: नियन्त्र, "controller/regulator") is a **local-first AI operations dashboard** — a single binary that gives developers complete visibility into their AI tool ecosystem: quota states, subscription costs, usage patterns, and budget health.

> **"Am I ready to code?"** — auto-detected quota readiness across accounts
>
> **"What am I spending?"** — unified view of every AI subscription with budget alerts
>
> **"What happened?"** — full activity log with provenance for every data point

## The Problem

### The AI Tool Sprawl

A modern developer in 2025–2026 subscribes to 5–15 AI services simultaneously:

| Category | Examples | Billing Model |
|----------|----------|--------------|
| AI Coding | Antigravity, Cursor, Copilot, Claude Code, Codex | Monthly with rolling quota windows |
| AI Chat | ChatGPT, Claude, Gemini, Perplexity | Monthly with message limits |
| AI API | OpenAI API, Anthropic API, Google AI Studio | Pay-as-you-go (unbounded) |
| AI Creative | Midjourney, Runway, ElevenLabs, Suno | Monthly with credit limits |
| AI Productivity | Notion AI, Grammarly | Per-seat monthly |

Each tool has its own dashboard, its own billing page, its own quota display, its own renewal date. There is **no unified view**.

### The Daily Friction

1. **Quota blindness** — You exhaust quota on one AI coding tool, switch to another, but don't know if *that* account has quota either
2. **Subscription amnesia** — At $20-200/month each, AI tools silently accumulate into a significant monthly expense
3. **Renewal surprise** — Tools renew at different dates; a $200 ChatGPT Pro renewal catches you off guard
4. **Trial rot** — Free trials expire silently, converting to paid subscriptions
5. **Cost invisibility** — Pay-as-you-go APIs (OpenAI, Anthropic) have no natural ceiling

### Why Existing Solutions Fall Short

| Approach | What it does well | What it doesn't do |
|----------|-------------------|-------------------|
| **Quota trackers** (e.g. onWatch, CodexBar) | Multi-provider API quota monitoring (9-16+ providers), historical trends, burn rate projections | No subscription management, no budget forecasting, no MCP, no multi-account routing |
| **Subscription trackers** (e.g. Wallos) | Generic subscription tracking with 10+ notification channels | No AI quota awareness, no usage analytics, manual entry only |
| **Account managers** (e.g. cockpit-tools, Antigravity-Manager, ag-manager) | Multi-account switching, auto-rotation, local API proxy | High T&S risk — programmatic switching triggers account suspension |
| **Enterprise SaaS platforms** (CloudEagle, Torii) | Team license management, compliance | $10K+/yr, team-focused, not for individual developers |
| **LLM observability** (Langfuse, Helicone, Portkey) | API-level trace logging, cost attribution | For apps *building with* LLMs, not developers *using* AI coding tools |
| **IDE extensions** (Windsurf Quota, Claude Quota Tracker) | Quick status bar indicator for one provider | Single-provider, no history, no budget context, ephemeral |

### Market Position

Niyantra sits in a gap between these categories: **quota monitoring + subscription management + budget intelligence + AI agent integration** in a single local-first tool.

Based on a 28-tool competitive analysis across 8 market categories:

- **Quota trackers** like **onWatch** (600+ stars, Go+SQLite) and **CodexBar** (10+ providers) lead on provider breadth but have zero subscription management, zero MCP, and single-account tracking only.
- **CLI tools** like **caut** (Rust, 16+ providers) generate agent-ready JSON output but lack persistence, dashboards, and cost intelligence.
- **Subscription trackers** like **Wallos** (4K+ stars) excel at renewal management with 10+ notification channels but know nothing about AI quotas.
- **Nobody** combines quota monitoring + subscription economics + AI agent integration.

**Niyantra leads with 37+ features** in the competitive matrix — next closest is onWatch at 12/37. Our unique moats:
1. **Multi-account observability** (28+ accounts, passive read-only) — unlike the 6+ account managers that support multi-account via risky active switching, Niyantra monitors all accounts without triggering T&S
2. **MCP Server** (11 tools, stdio + Streamable HTTP) — completely uncontested, zero competitors
3. **Combined quota + subscription + budget** in one tool — nobody else bridges this
4. **Quad-channel notifications** (OS + SMTP + Webhook + WebPush) — more channels than any competitor
5. **7 providers** (Antigravity, Codex, Claude deep, Cursor, Gemini CLI, Copilot, Manual) — broadest coverage in the local-first category

**Niyantra's thesis:** Knowing your quota is only half the problem. You also need to know what you're spending, when renewals hit, which account to switch to, and your AI agents need this context too.

## The Solution

### Multi-Source Data Capture

Niyantra gathers data from multiple sources, each with its own capture method:

```
┌─────────────────────────────────────────────────────────┐
│                   DATA SOURCES (7 Providers)             │
│                                                         │
│   Antigravity LS ──── Connect RPC to local LS ──────┐   │
│   Claude Code    ──── JSONL log parsing + bridge ────┤   │
│   Codex/ChatGPT  ──── OAuth API polling ─────────────┤   │
│   Cursor         ──── session token → HTTP API ──────┤   │
│   Gemini CLI     ──── OAuth → GCP APIs ──────────────┤   │
│   GitHub Copilot ──── PAT → billing API ─────────────┤   │
│   Manual Entry   ──── subscription form ─────────────┤   │
│                                                      │   │
│                                                      ▼   │
│                              ┌─────────────────────┐     │
│                              │  SQLite Ledger      │     │
│                              │  (18 tables, v18)   │     │
│                              │                     │     │
│                              │  snapshots           │     │
│                              │  subscriptions       │     │
│                              │  activity_log        │     │
│                              │  config (74 keys)    │     │
│                              │  data_sources        │     │
│                              │  token_usage_daily   │     │
│                              │  webpush_subscriptions│    │
│                              └─────────┬───────────┘     │
│                                        │                 │
│                    ┌───────────────────▼──────────┐      │
│                    │  Dashboard / CLI / MCP       │      │
│                    │  (60 API endpoints)          │      │
│                    └─────────────────────────────┘       │
└─────────────────────────────────────────────────────────┘
```

### Provenance Guarantee

Every data point stored by Niyantra is tagged with:
- **Who** captured it (which account/email)
- **How** it was captured (manual vs auto)
- **Where** it was captured from (CLI, dashboard, watch daemon, log parser)
- **When** it was captured (timestamp)

This means the user can always verify: "Was this data captured by me clicking a button, or by a background process?" — critical for trust when autonomous capture is enabled.

### The Ledger Model

Niyantra maintains a local SQLite database — a **ledger** of quota snapshots tagged by account and provenance:

```
┌────────────────────────────────────────────────────────────┐
│  SNAPSHOT #147                                              │
│  Account:   work@company.com                                │
│  Captured:  2026-04-17 00:30:00 UTC                        │
│  Method:    manual  │  Source: ui  │  Via: antigravity      │
│  Plan:      Pro                                             │
│  ┌──────────────┬──────────┬─────────────────┐             │
│  │ Quota Group   │ Remaining│ Resets At        │             │
│  ├──────────────┼──────────┼─────────────────┤             │
│  │ Claude + GPT  │ 40%      │ 2026-04-17 04:24│             │
│  │ Gemini Pro    │ 100%     │ 2026-04-17 04:48│             │
│  │ Gemini Flash  │ 100%     │ 2026-04-17 04:48│             │
│  └──────────────┴──────────┴─────────────────┘             │
└────────────────────────────────────────────────────────────┘
```

### Readiness Prediction

From the ledger, Niyantra computes **readiness** — a purely local calculation that tells you, right now, which account is usable and which is exhausted, without making any network calls.

## Design Principles

### 1. Local-First

All data stays on your machine. All computation happens from the local SQLite database. `niyantra status` makes **zero** network calls. The tool works offline, works instantly, and never phones home.

**Why:** Privacy. Speed. No dependency on external services. Your AI spending data is yours alone.

### 2. Manual by Default, Auto Opt-In

Niyantra starts in **manual mode** — data enters the system only when you explicitly trigger it. Auto-capture (polling, log parsing) is available but must be explicitly enabled. Manual capture is always allowed regardless of mode.

**Why:** Predictability. Users must trust that the tool does exactly what they asked — no more. When polling is enabled, every auto-captured data point is tagged `method=auto` so you can always distinguish it from manual captures.

### 3. Single Binary

Go compiles to a single static binary with the web dashboard embedded via `embed.FS`. No runtime dependencies, no package managers, no containers, no `node_modules`.

**Why:** Copy it, run it, done. Trivially portable, trivially deployable, trivially auditable.

### 4. Full Provenance

Every snapshot, every config change, every subscription mutation is logged in the activity log. The system maintains a complete audit trail of what happened, when, and how.

**Why:** When auto-capture exists alongside manual capture, users need proof that their data wasn't silently modified by a background process they didn't authorize.

### 5. Multi-Platform

Process detection works on Windows (CIM/PowerShell/WMIC), macOS (ps/lsof), and Linux (ps/ss/netstat). The same binary runs everywhere Go compiles.

## Capture Modes

| Mode | Manual Snap | Auto-Poll | Log Parsing | Use Case |
|------|-------------|-----------|-------------|----------|
| **Manual** (default) | ✅ Always | ❌ Blocked | ❌ Blocked | Privacy-first, event-driven usage |
| **Auto** (opt-in) | ✅ Always | ✅ Per-source | ✅ Per-source | "I want Niyantra to keep itself updated" |

Manual snaps are **always** allowed regardless of mode. The auto-capture toggle only controls autonomous background capture. This prevents the "I locked myself out" footgun.

## Data Sources

### Current (Implemented — 7 Providers)
- **Antigravity Language Server** — quota snapshots via local Connect RPC API. Handles protobuf `*float64` semantics for `remainingFraction`. Users can fine-tune stale LS cache values post-snap via Quick Adjust (±5%/±10%).
- **Claude Code** — real-time rate limit data via statusline file bridge + deep JSONL session parsing for per-turn token analytics (input/output/cache) with model-aware cost estimation. New `internal/claude/` package (refactored from claudebridge).
- **Codex / ChatGPT** — OAuth API polling with proactive token refresh, multi-quota tracking (5h window, 7d window, code review). Credentials from `~/.codex/auth.json`, account identity via JWT `id_token` parsing with OIDC name + picture extraction.
- **Cursor** — Session token detection from filesystem, HTTP API polling to `cursor.com/api/usage` for request counts + USD credit balance. Supports legacy request-based and new credit-based billing models.
- **Gemini CLI** — OAuth credential discovery from `~/.config/gemini/`, 2-step API (loadCodeAssist + retrieveUserQuota) for rate limit tracking.
- **GitHub Copilot** — GitHub Personal Access Token → billing API for usage tracking. PAT masked in API responses.
- **Manual Subscriptions** — 26 platform presets with expert-curated notes

Each source is registered in a `data_sources` table with its own configuration. Adding a new source requires no schema changes — just a new row and a Go handler.

## Workflow

### Daily Usage (Manual Mode)

```
Morning:
  1. Open IDE → Antigravity extension connects
  2. Run: niyantra snap → capture quota state
  3. Start coding

Hit quota wall:
  4. Open dashboard or run: niyantra status
  5. See which account has quota → switch in IDE
  6. Run: niyantra snap → capture new state
  7. Continue coding

End of day:
  8. Check Overview tab → review today's spend
```

### Power User (Auto Mode)

```
Setup:
  1. niyantra serve
  2. Settings → Auto Capture → ON
  3. Enable: Antigravity (poll every 5 min)
  4. Enable: Claude Code (watch log files)

Daily:
  - Niyantra auto-updates quota state in background
  - Activity log shows every capture with source tags
  - Mode badge shows 🟠 Auto in header
  - Manual snap still works for instant capture
```

### Dashboard

Four-tab dashboard at `http://localhost:9222` (also deployed as PWA at `https://niyantra.bhaskarjha.com`):

- **Quotas** — provider-sectioned layout (Antigravity / Codex / Claude / Cursor / Gemini / Copilot), per-model progress bars with reset timers, provider filter dropdown, status filter (Ready / Low / Empty), tag filter, text search, split-button snap (Snap Now / Snap All Sources), twin-axis history chart, activity heatmap, AI Credits tracking, Quick Adjust
- **Subscriptions** — hybrid card + provider layout with spend summary bar, search, 26 platform presets, CSV export, platform filter, status filter
- **Overview** — monthly budget vs actual, switch advisor, provider health cards, estimated cost tracking, Git commit costs, sessions timeline, renewal calendar, spending breakdown, token usage analytics, JSON/CSV export
- **Settings** — capture config (7 providers), budget, model pricing, notifications (4 channels), data sources, import/export, activity log, keyboard shortcuts, command palette (`Ctrl+K`)

## Roadmap

### ✅ Phase 1–3: Foundation
Schema, subscription CRUD, 26 presets, overview stats, CSV export.

### ✅ Phase 4: History & Insights
Chart.js quota history, budget thresholds, smart insights engine.

### ✅ Phase 5: Settings & Polish
Settings tab, search, keyboard shortcuts, PWA manifest.

### ✅ Phase 5.5: Infrastructure Overhaul
Config system (SQLite), activity log, snapshot provenance, data sources registry, mode badge.

### ✅ Phase 6: Auto-Capture
Polling agent with ticker loop, exponential backoff, config-driven enable/disable, graceful shutdown.

### ✅ Phase 7: Cycle Tracking & Intelligence
Per-model reset cycle detection (3 methods), usage rate forecasting, projected exhaustion, budget burn rate alerts.

### ✅ Phase 8: MCP Server
MCP server over stdio (11 tools) for AI agent integration. Uses official Go SDK (github.com/modelcontextprotocol/go-sdk).

### ✅ Phase 9: Multi-Source & Safety Net
- **Claude Code statusline bridge** — real-time rate limit data via shared file, auto-patched `~/.claude/settings.json`, 5h/7d usage meters
- **OS Notifications (quad-channel: OS + SMTP + Webhook + WebPush)**
- **Application Layer Architecture**:
  - `agent/`: polling loop + session management
  - `client/`: LS detection + quota fetch (Connect RPC)
  - `codex/`: OAuth + Codex API polling + OIDC JWT parsing
  - `claude/`: deep session parser + statusline bridge
  - `cursor/`: session token auth + HTTP API polling
  - `gemini/`: OAuth + GCP billing/quota APIs
  - `copilot/`: GitHub PAT + Copilot billing endpoints
  - `advisor/`: switch recommendation engine
  - `tracker/`: cycle detection + intelligence + sessions
  - `readiness/`: pure readiness computation
  - `notify/`: quad-channel notifications
  - `forecast/`: cost + TTX forecasting
  - `costtrack/`: blended model pricing
  - `tokenusage/`: Claude Code JSONL token analytics
  - `gitcorr/`: git commit ↔ token usage cost correlation
- **Database backup/restore** — CLI `backup`/`restore` commands + one-click dashboard download, schema validation on restore
- **Command palette** — `Ctrl+K` modal with fuzzy search, keyboard navigation, 12+ actions
- **Schema v5** — `claude_snapshots` table, config keys: `claude_bridge`, `notify_enabled`, `notify_threshold`

### ✅ Phase 10: Intelligence & Insights
- **Smart switch advisor** — cross-account routing engine: ranks accounts by remaining% (60%), burn rate (20%), time-to-reset (20%). Actions: "switch", "stay", "wait". New `internal/advisor/` package.
- **MCP insight tools** — `analyze_spending` (spending analysis, savings detection, category breakdown) + `switch_recommendation` (wraps advisor for AI agents). Total: 7 MCP tools.
- **Enhanced subscription insights** — structured insights with type/severity/icon: unused detection (30+ days), imminent renewal (3 days), spending anomaly (2× budget), category overlap (3+ subs)
- **Renewal calendar** — CSS grid month-view calendar with pin markers on renewal dates, month navigation, legend
- **JSON export** — `GET /api/export/json` with full data portability (accounts, subs, snapshots, claude data, config)
- **System alerts** — persistent dismissible banners for quota warnings, budget overages, bridge errors (schema v6: `system_alerts` table)
- **Data retention cleanup** — enforce `retention_days` config via agent poll hook

### ✅ Phase 11: Codex & Sessions
- **Codex API integration** — OAuth polling with proactive token refresh, multi-quota tracking (5h, 7d, code review), credential detection from `~/.codex/auth.json`, account identity via JWT. New `internal/codex/` package.
- **Session detection** — usage-change-based sessions with configurable idle timeout, integrated into polling agent for all 3 providers (Antigravity, Codex, Claude). New `internal/tracker/session.go`.
- **JSON import** — additive merge strategy with natural-key deduplication (accounts, subs, snapshots). Completes data portability loop.
- **Usage logs** — manual per-subscription usage tracking with summary aggregation.
- **Codex MCP tool** — `codex_status` for AI agents. Total: 8 MCP tools.
- **Dashboard** — Codex status card (Overview), Codex settings toggle, sessions timeline, import button, 2 new command palette entries.
- **Schema v7** — `codex_snapshots`, `usage_sessions`, `usage_logs` tables + config keys: `codex_capture`, `session_idle_timeout`.

### ✅ Phase 11.5: Hardening & Polish
- **MIT License** — fully independent codebase, ready for public release
- **Makefile** — build/run/test/vet/clean/demo targets with `-ldflags` version injection
- **`niyantra demo`** — seed command populates realistic sample data (2 accounts, 24 snapshots, 5 subs)
- **README redesign** — from 343-line technical manual to ~170-line hero + quickstart + features format with badges
- **ARCHITECTURE.md rewrite** — fixed single-line encoding issue, updated to schema v7
- **Go unit tests** — 16 tests (readiness + advisor packages), all passing
- **GitHub Actions** — CI (build+vet+test on 3 OS) + release (multi-arch binaries on tag push)
- **Code independence audit** — zero references to prior art in tracked files or git history

### ✅ Phase 12: UX Overhaul
- **Provider-sectioned Quotas** — Antigravity, Codex, Claude Code shown in dedicated collapsible sections with provider-specific headers and color coding
- **Provider filter dropdown** — filter by provider (All / Antigravity / Codex / Claude)
- **Status filter** — filter accounts by readiness state (Ready / Low / Empty) with provider-aware logic
- **Split-button snap** — primary "Snap Now" (current account) + secondary "Snap All Sources" (all providers)
- **Hybrid subscription layout** — card + provider grouping with inline spend summary bar
- **Provider health cards** — overview tab shows per-provider health status
- **Codex OIDC enhancement** — extract display name + profile picture from JWT `id_token` claims
- **Visual polish** — hover accents, UUID truncation, time-ago columns, dynamic advisor labels, quick links deduplication
- **Chart.js bundled locally** — removed CDN dependency, Chart.js served from embedded assets
- **Schema v8** — `ai_credits_json` column on snapshots for native Google AI Credits tracking
- **Schema v9** — `email` column on `codex_snapshots` for multi-account Codex identity

### ✅ Phase 12.5: Data Integrity & Stability
- **Quick Adjust** — manual quota correction at group and model level; `PATCH /api/snap/adjust` endpoint with persistent DB updates
- **Protobuf `*float64` handling** — correct handling of `remainingFraction` where protobuf zero values mean 0% (fully exhausted), not null (missing data)
- **Reset-time-corrected aggregation** — group-level quota calculations (Claude+GPT) use reset-time-adjusted model values instead of raw snapshot data
- **Readiness-based dimming** — accounts dimmed by `isReady` flag instead of `allExhausted`, ensuring consistency across new and stale data
- **Collapse state persistence** — provider section collapse/expand state baked into HTML generation, preventing flash-expand on filter change
- **Subscription tab pre-loading** — subscription data loaded on init, eliminating white flash on tab switch
- **Tab animation removed** — `tabFadeIn` CSS animation removed (was causing background flickering during DOM re-paints)

### ✅ Phase 13: Foundation Sprint — ~9 days
- **Account notes + tags** — per-account metadata with predefined palette + custom tags (schema v10)
- **Live poll interval reload** — poll interval read inside ticker loop, not just at startup
- **Pinned/favorite model** — star one group per account, shows in collapsed view
- **Tag-based filtering** — filter accounts by tag in Quotas toolbar
- **Model pricing config** — per-model $/1M token pricing stored in config (prerequisite for cost tracking)
- **Notification wiring** — connect existing `notify/` engine to polling loop with threshold alerts
- **Quota time-to-exhaustion** — linear regression burn rate forecasting with severity badges
- **Estimated cost tracking** — quota delta × model pricing = estimated spend
- **Credit renewal day** — per-account renewal tracking with countdown badges (schema v11)
- **Frontend modularization** — monolithic 4,265-line `app.js` decomposed into 27 strict-mode TypeScript modules bundled via esbuild (IIFE format, 89 KB minified)

### ✅ Phase 14: Competitive Parity Sprint
- **Activity heatmap** — GitHub-style 365-day contribution grid from existing snapshot data
- **Claude Code: deep tracking** — full JSONL session parser for per-turn token analytics (input/output/cache) with model-aware cost estimation. New `internal/claude/` package (refactored from `claudebridge/`).
- **Provider: Cursor** — session token → HTTP API (`cursor.com/api/usage`) for quota/usage data. Supports legacy request-based and new USD credit billing models.
- **Provider: Gemini CLI** — OAuth + GCP APIs for rate limit tracking
- **Docker deployment** — `Dockerfile` + `docker-compose.yml` for self-hosted deployment

### ✅ Phase 15: Deep Analytics Sprint
- **Token usage analytics** — parse Claude JSONL sessions for per-conversation token costs
- **Git commit correlation** — cost per feature branch (unique — no competitor does this)
- **Streamable HTTP MCP** — remote agent access over HTTP transport (11 tools)
- **Provider: GitHub Copilot** — GitHub PAT → billing endpoints

### 🔲 Phase 16: Ecosystem & Growth — in progress
- ✅ **SMTP/Email notifications** — pure Go SMTP with TLS/STARTTLS, HTML templates
- ✅ **Webhook notifications** — Discord, Telegram, Slack, ntfy/Gotify/generic
- ✅ **WebPush notifications** — VAPID (RFC 8292) + RFC 8291 encryption, zero x/crypto dependency
- **Cloud sync (Pro tier)** — encrypted multi-device sync via PocketBase
- **Plugin system** — `DataSource` interface for custom provider integrations
- **Context window dashboard** — visualize IDE context consumption (requires LS research)

> **Full details:** The internal development roadmap (`draft/roadmap.md`) contains 22 features with quantified scoring across Gap, Value, Effort, and Moat dimensions, plus a 37-feature × 12-tool competitive comparison matrix. Currently at 37+/37 features shipped.

## Real-World Use Cases

### The Quota Emergency
You're deep in a coding flow. Claude hits 0% mid-task. You have 3 Antigravity accounts. Which one has quota? Open Niyantra dashboard -- the readiness grid tells you in 1 second. The switch advisor says "switch to personal@gmail.com (85% remaining, score 78)."

### The Expense Report
End of month, your manager asks "how much are we spending on AI tools?" Open Subscriptions tab, click CSV Export. Done. Total monthly spend, per-platform breakdown, renewal dates -- all in one file.

### The Trial Trap
You signed up for Cursor Pro trial (14 days), Midjourney trial (7 days), and Claude Pro trial (free month). Which one converts to paid next? The renewal calendar shows Midjourney in 2 days with a pin marker. You cancel before the charge.

### The Agent Handoff
You're using Claude Desktop with MCP. Mid-conversation, Claude asks your Niyantra MCP server: "What's my budget status?" Niyantra responds: "$127 of $150 spent, 15 days remaining. Burn rate suggests $190 by month end." Claude adjusts its recommendations accordingly.

### The Multi-Account Rotation
You have work and personal Antigravity accounts. Auto-capture polls both every 60 seconds. At 3am, your work account's Claude quota resets. Next morning, the activity log shows the exact reset time -- you know you're starting fresh.

## Non-Goals

Things Niyantra deliberately does **not** do:

- **Programmatic account switching** — Niyantra reads state, it doesn't write it. Programmatic switching via `registerGdmUser` or similar RPC calls triggers Google's AI-driven Trust & Safety systems, which can result in **immediate account suspension** (Gmail, Drive, everything — not just the IDE). Tools like AG Switchboard, cockpit-tools, and windsurf-assistant take this risk — we deliberately don't.
- **Account pool rotation** — Tools like WindsurfPoolAPI pool multiple accounts for load balancing. Same T&S risk, amplified at scale.
- **Proactive token refresh** — Aggressive automated token management is flagged as botting behavior by Google's anomaly detection.
- **IDE extension / status bar** — Niyantra is a standalone dashboard, not an IDE plugin. The macOS menu bar space is saturated (CodexBar, ClaudeBar, AgentBar, TokenBar). We complement, not compete.
- **Payment processing** — This is a consumer tracker, not a billing system
- **Multi-user/team** — Single-user, single-machine design (cloud sync is optional, not multi-tenant)
- **API gateway/proxy** — We monitor usage, we don't route or block requests. LiteLLM, Portkey, OmniRoute serve that need.

## Cloud Architecture (Planned)

Niyantra has a designed (not yet implemented) cloud tier for optional multi-device sync:

- **Domain:** `niyantra.bhaskarjha.com` (PWA already deployed via Cloudflare Pages)
- **Backend:** PocketBase on Oracle Cloud ARM instance
- **Auth:** Google OAuth with device binding
- **Sync:** End-to-end encrypted, conflict-free merge logic
- **Mobile:** PWA-first approach with push notifications
- **Monetization:** Free (local-only) / Pro (cloud sync + push + priority support)

Full architecture documented in 8 internal design documents covering auth, sync, schema, desktop client, backend, phases, mobile, and monetization.

> **Note:** The free tier will always be fully functional local-only. Cloud features are additive, never required.

## Success Metrics

Niyantra is successful when:

1. **< 3 seconds** from `niyantra snap` to "snapshot stored"
2. **< 100ms** from `niyantra status` to "readiness displayed"
3. **Every data point traceable** — method, source, timestamp on every snapshot
4. **1 API call** per manual snap — no more, ever
5. **< 20 MB** binary size (includes embedded SQLite engine + web assets + Chart.js)
6. **Zero surprise captures** — auto mode only when explicitly enabled
7. **Multi-source** — 7 AI coding tools tracked in a unified view ✅
8. **7 providers** shipped: Antigravity + Codex + Claude deep + Cursor + Gemini CLI + Copilot + Manual ✅
9. **37+ features** shipped across Phases 1-16, closing all competitive gaps vs onWatch ✅
10. **148 tests** across 10 packages, all passing ✅
11. **4 notification channels**: OS + SMTP + Webhook + WebPush ✅
12. **11 MCP tools** (stdio + Streamable HTTP) ✅
