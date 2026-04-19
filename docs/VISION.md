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

### Why Existing Solutions Fail

| Approach | Problem |
|----------|---------|
| **Check each tool's UI** | 10 tabs, 10 logins, 10 different UIs |
| **Spreadsheets** | Manual data entry, stale immediately |
| **Enterprise billing tools** (Stripe, Chargebee) | For SaaS *providers*, not *consumers* |
| **Mint/YNAB** | Track bank transactions, not API quotas or rolling windows |
| **Background polling** | Hammers APIs, risks rate limiting, wastes resources |

**There is no local-first, developer-focused tool for tracking AI subscriptions from the consumer side.** Niyantra fills this gap.

## The Solution

### Multi-Source Data Capture

Niyantra gathers data from multiple sources, each with its own capture method:

```
┌─────────────────────────────────────────────────────────┐
│                   DATA SOURCES                           │
│                                                         │
│   Antigravity LS ──── manual snap / auto-poll ────┐     │
│   Claude Code    ──── local JSONL log parsing ────┤     │
│   Codex          ──── local JSONL log parsing ────┤     │
│   Manual Entry   ──── subscription form ──────────┤     │
│   (Future: APIs) ──── API polling ────────────────┤     │
│                                                    │     │
│                                                    ▼     │
│                              ┌─────────────────────┐     │
│                              │  SQLite Ledger      │     │
│                              │                     │     │
│                              │  snapshots          │     │
│                              │  subscriptions      │     │
│                              │  activity_log       │     │
│                              │  config             │     │
│                              │  data_sources       │     │
│                              └─────────┬───────────┘     │
│                                        │                 │
│                    ┌───────────────────▼──────────┐      │
│                    │  Dashboard / CLI / MCP       │      │
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

### Current (Implemented)
- **Antigravity Language Server** — quota snapshots via local Connect RPC API
- **Claude Code Bridge** — real-time rate limit data via statusline file bridge. Auto-patches `~/.claude/settings.json` with a bash snippet that pipes stdin to `~/.niyantra/data/anthropic-statusline.json`. Zero API calls, zero dependencies.
- **Codex / ChatGPT** — OAuth API polling with proactive token refresh, multi-quota tracking (5h window, 7d window, code review). Credentials from `~/.codex/auth.json`, account identity via JWT `id_token` parsing. (Phase 11)
- **Manual Subscriptions** — 26 platform presets with expert-curated notes

### Planned
- **Codex local logs** — parse `~/.codex/*.jsonl` for offline session/token usage
- **OpenAI/Anthropic APIs** — query usage endpoints with user-provided API keys
- **Copilot/Gemini** — API-based quota tracking

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

Four-tab dashboard at `http://localhost:9222`:

- **Quotas** — readiness grid, per-model detail, history chart
- **Subscriptions** — card-based view, 26 presets, search, filters
- **Overview** — budget alerts, switch advisor, intelligence insights, renewal calendar, Codex status card, sessions timeline, spend breakdown, JSON/CSV export
- **Settings** — capture config (Antigravity + Codex + Claude), budget, data sources, import/export, activity log, shortcuts

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
MCP server over stdio (8 tools) for AI agent integration. Uses official Go SDK (github.com/modelcontextprotocol/go-sdk).

### ✅ Phase 9: Multi-Source & Safety Net
- **Claude Code statusline bridge** — real-time rate limit data via shared file, auto-patched `~/.claude/settings.json`, 5h/7d usage meters
- **OS-native notifications** — cross-platform notification engine (`powershell`, `osascript`, `notify-send`) with once-per-cycle guard
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

### 🔲 Phase 12: Remote & Enterprise (NEXT)
- **Streamable HTTP MCP transport** — remote agent access (SSE is deprecated; modern MCP requires Streamable HTTP)
- **SMTP/Email notifications** — enterprise notification channel with TLS/STARTTLS support
- **Multi-machine sync** — encrypted export/import with merge logic

### 🔲 Phase 13+: Ecosystem
- Timeline view across all data sources
- Git commit correlation (ROI tracking — cost per feature shipped)
- Plugin system for custom data sources
- WebPush notifications (VAPID)
- Copilot/Gemini API integration

## Non-Goals

Things Niyantra deliberately does **not** do:

- **Cloud sync** — Data stays local, period. No accounts, no SaaS.
- **Account switching** — Niyantra reads state, it doesn't write it
- **Payment processing** — This is a consumer tracker, not a billing system
- **Multi-user/team** — Single-user, single-machine design
- **API gateway/proxy** — We monitor usage, we don't route or block requests

## Success Metrics

Niyantra is successful when:

1. **< 3 seconds** from `niyantra snap` to "snapshot stored"
2. **< 100ms** from `niyantra status` to "readiness displayed"
3. **Every data point traceable** — method, source, timestamp on every snapshot
4. **1 API call** per manual snap — no more, ever
5. **< 20 MB** binary size (includes embedded SQLite engine + web assets)
6. **Zero surprise captures** — auto mode only when explicitly enabled
7. **Multi-source** — at least 2 AI coding tools tracked in a unified view
