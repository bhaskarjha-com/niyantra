# Niyantra

**Local-first AI operations dashboard.**

Niyantra (Sanskrit: नियन्त्र, "controller") is a single-binary dashboard that gives developers complete visibility into their AI tool ecosystem. It **auto-captures Antigravity quotas** from the local language server, **tracks Codex/ChatGPT usage** via OAuth API, **monitors Claude Code rate limits** via statusline bridge, **tracks subscriptions** for 26+ AI platforms, and provides **budget alerts, usage insights, switch recommendations, and a full activity log** with provenance on every data point.

> **Zero daemon by default. Full provenance. All local.**

## The Problem

Developers in 2026 juggle **8+ AI subscriptions** — Antigravity, Cursor, Claude, ChatGPT, Copilot, API credits. Each has different limits, billing cycles, and quota windows. You waste time:

- Checking each tool's dashboard individually
- Forgetting which account has quota right now
- Missing trial expiry dates (and getting auto-billed)
- Manually tallying AI spending for expense reports

## The Solution

```
niyantra snap       # Captures current account's quota (1 API call)
niyantra status     # Shows all accounts' readiness (0 API calls)
niyantra serve      # Launches visual dashboard at http://localhost:9222
niyantra mcp        # Starts MCP server for AI agent integration
```

Each `snap` makes exactly one HTTP call to the **local** Antigravity language server (already running via your IDE). No cloud APIs, no API keys, no rate limiting risk.

## Prerequisites

- **Go 1.22+** — to build from source
- **Antigravity IDE** — must be running (it provides the language server)
- **One account logged in** — `snap` captures whichever account is currently active

## Quick Start

```bash
# 1. Build
go build -o niyantra ./cmd/niyantra

# 2. Log into your first Antigravity account in the IDE

# 3. Capture its quota
./niyantra snap

# 4. Switch to another account in the IDE, then:
./niyantra snap

# 5. See which account is ready
./niyantra status

# 6. Or launch the visual dashboard
./niyantra serve
# → open http://localhost:9222
```

## Dashboard — 4 Tabs

The dashboard at `http://localhost:9222` has four tabs:

### Quotas Tab (Auto-Tracked)

Real-time readiness grid for Antigravity accounts:

| Account | Claude+GPT | Gemini Pro | Gemini Flash | Status |
|---------|-----------|-----------|-------------|--------|
| work@company.com | 40% ↻3h | 100% ↻4h | 100% ↻4h | ✅ Ready |
| personal@gmail.com | 0% ↻1h | 100% ↻2h | 100% ↻4h | ⚠️ Low |

- **Click any row** → expands per-model detail (progress bars, reset countdowns)
- **Snap Now** → captures current account's quota
- **Quota History Chart** — Chart.js line chart showing quota trends over time
  - Filter by account, view last 20/50/100 snapshots
  - Theme-aware (adapts to dark/light mode)
- Color-coded by group (orange = Claude+GPT, green = Gemini Pro, blue = Flash)

### Subscriptions Tab (Manual Tracking)

Card-based view of all AI subscriptions:

- **26 platform presets** — one-click onboarding for Claude, ChatGPT, Cursor, Copilot, Midjourney, etc.
- **Pre-filled expert tips** — "Claude's 5h window is rolling, not fixed", "Cursor Auto mode is unlimited"
- **Status badges** — Active, Trial, Paused, Cancelled
- **Trial countdown** — "Trial ends in 3 days" with red badge
- **Dashboard links** — one-click to each tool's billing/usage page
- **Status page links** — "Is it down?" → opens status.anthropic.com, etc.
- **Search** — real-time full-text search across all subscriptions
- **Filters** — by status and category
- **Category grouping** — coding, chat, API, image, audio, productivity
- **Auto-link** — when `snap` detects an Antigravity account, auto-creates a subscription card

### Overview Tab

- **Budget alert** — ok/warning/danger states when spend approaches your monthly budget
- **Smart switch advisor** — cross-account routing: "switch", "stay", or "wait" with scores
- **Codex / ChatGPT card** — multi-quota bars (5h window, 7d window, code review) with manual snap
- **Sessions timeline** — usage sessions across all providers with live indicators
- **Smart insights** — trial warnings, top spending category, renewal alerts, annual savings potential
- **Monthly/annual spend** with category breakdown
- **Renewal calendar** — month-view with pin markers on renewal dates
- **Budget forecast** — burn rate per day, projected monthly spend, on-track/over-budget
- **System alerts** — persistent dismissible banners for quota warnings
- **CSV/JSON Export** — download all data for tax/expense reports or data portability

### Settings Tab

- **Capture & Sources** — auto-capture toggle, poll interval, auto-link toggle, data sources list
- **Claude Code Bridge** — statusline integration for real-time Claude Code rate limits (5h/7d windows)
- **Codex / ChatGPT** — enable Codex capture toggle with OAuth status display
- **Budget & Display** — monthly spending threshold with alerts, default currency, theme
- **Notifications** — OS-native desktop alerts when quota drops below threshold
- **Data Management** — snapshot retention, CSV/JSON export, database backup, JSON import (additive merge)
- **Activity Log** — structured event log with filters (snaps, config changes, server starts)
- **Keyboard shortcuts** — reference grid
- **About** — version, schema, mode, active sources

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `1` `2` `3` `4` | Switch tabs |
| `N` | New subscription |
| `S` | Snap quota |
| `/` | Search subscriptions |
| `Ctrl+K` | Command palette |
| `Esc` | Close modal |

### What It Does NOT Show

- **AI Credits** — not available via local language server API. Requires cloud API that the LS doesn't expose.

## CLI Commands

| Command | What it does | Network calls |
|---------|-------------|---------------|
| `niyantra snap` | Capture current account's quota | 1 (local LS) |
| `niyantra status` | Show all accounts' readiness | 0 |
| `niyantra serve` | Start web dashboard | 0 (+ 1 per snap) |
| `niyantra mcp` | Start MCP server for AI agents | 0 |
| `niyantra backup` | Create timestamped database backup | 0 |
| `niyantra restore <file>` | Restore database from backup | 0 |
| `niyantra version` | Print version | 0 |

### Flags

```
--port     Dashboard port (default: 9222)
--db       Database path (default: ~/.niyantra/niyantra.db)
--auth     HTTP basic auth for dashboard (user:pass)
--debug    Enable verbose logging
```

## Design Principles

1. **Zero daemon by default** — No background process, no polling, no system tray. Manual capture is always available. Auto-capture is opt-in.
2. **Full provenance** — Every data point tagged with who captured it, how, and from which source.
3. **Local-first** — All readiness computation from SQLite, zero network. `status` and `serve` never phone home.
4. **Single binary** — Go with embedded web assets via `embed.FS`. No Node.js, no npm, no containers.
5. **Multi-platform** — Process detection works on Windows (CIM/PowerShell), macOS (ps/lsof), Linux (ps/ss).

## How It Works

```
niyantra snap
    │
    ▼
┌─────────────────────────┐
│  1. Detect Language Server │  ← find process by name, extract CSRF token
│     (ps / CIM / netstat)   │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  2. Fetch Quotas           │  ← 1 POST to localhost (Connect RPC)
│     GetUserStatus          │  ← returns 6 models + plan info
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  3. Store in SQLite        │  ← ~/.niyantra/niyantra.db
│     snapshots + accounts   │  ← models_json + raw_json
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  4. Compute Readiness      │  ← group models, calc percentages
│     (pure local, no I/O)   │  ← reset countdowns, staleness
└─────────────────────────┘
```

## What Data We Can Access

The Antigravity language server (running locally) exposes **one useful endpoint**: `GetUserStatus`, which returns:

| Field | Example | Shown in Dashboard |
|-------|---------|--------------------|
| `email` | `user@example.com` | ✅ Account name |
| `name` | `"Bhaskar Jha"` | ❌ Available but unused |
| `planName` | `"Pro"` | ✅ Plan badge |
| `availablePromptCredits` | `500` | ✅ Credits badge (✦ 500) |
| `monthlyPromptCredits` | `50000` | ❌ Available but unused |
| Per-model `label` | `"Claude Sonnet 4.6 (Thinking)"` | ✅ Model detail rows |
| Per-model `remainingFraction` | `0.4` (= 40%) | ✅ Progress bars |
| Per-model `resetTime` | `"2026-04-17T01:36:50Z"` | ✅ Reset countdown |
| Per-model `modelOrAlias.model` | `"MODEL_PLACEHOLDER_M35"` | ❌ Internal, not shown |

### What We CANNOT Access

The **"AI Credits: 1000"** shown in Antigravity's Agent Manager is served by a **different API** (Codeium cloud), not the local language server. We probed 24 potential RPC endpoints on the local LS — all returned 404. This data requires authentication against `codeium.com`, which is outside our scope.

## Project Structure

```
niyantra/
├── cmd/niyantra/              # CLI entrypoint + command dispatch
│   └── main.go                # snap, status, serve, mcp, version, backup, restore
│
├── internal/
│   ├── agent/                # Auto-capture polling (Phase 6+11)
│   │   ├── agent.go          # PollingAgent: multi-source polling + session managers
│   │   └── manager.go        # Start/Stop lifecycle
│   │
│   ├── client/                # Antigravity language server client
│   │   ├── client.go          # Detect + FetchQuotas (1 API call)
│   │   ├── detect_windows.go  # Windows: CIM → PowerShell → WMIC
│   │   ├── detect_unix.go     # macOS/Linux: ps aux
│   │   ├── ports.go           # Port discovery (lsof/ss/netstat)
│   │   ├── probe.go           # Connect RPC endpoint validation
│   │   ├── types.go           # API response structs + Snapshot
│   │   └── helpers.go         # Model grouping logic
│   │
│   ├── codex/                 # Codex/ChatGPT integration (Phase 11)
│   │   └── codex.go           # OAuth client, credential detection, usage API
│   │
│   ├── tracker/               # Cycle intelligence (Phase 7+11)
│   │   ├── tracker.go         # 3-method reset detection + cycles
│   │   ├── summary.go         # UsageSummary + BudgetForecast
│   │   └── session.go         # SessionManager: usage-change detection
│   │
│   ├── advisor/               # Switch Advisor (Phase 10)
│   │   └── advisor.go         # Stateless multi-factor account scoring
│   │
│   ├── mcpserver/             # MCP server (Phase 8+10+11)
│   │   └── mcpserver.go       # 8 tools over stdio for AI agents
│   │
│   ├── claudebridge/          # Claude Code bridge (Phase 9)
│   │   └── claudebridge.go    # Statusline patcher, rate limit reader
│   │
│   ├── notify/                # OS-native notifications (Phase 9)
│   │   └── notify.go          # Cross-platform toast, once-per-cycle guard
│   │
│   ├── store/                 # SQLite persistence (v7 schema)
│   │   ├── store.go           # Open, migrate (v1→v7), close
│   │   ├── snapshots.go       # Insert (with provenance), LatestPerAccount, History
│   │   ├── accounts.go        # GetOrCreateAccount (upsert by email)
│   │   ├── subscriptions.go   # Subscription CRUD, overview stats, renewals
│   │   ├── alerts.go          # System alerts CRUD (Phase 10)
│   │   ├── presets.go         # 26 platform preset templates
│   │   ├── config.go          # Server config CRUD (typed key-value)
│   │   ├── activity_log.go    # Structured activity log CRUD
│   │   ├── data_sources.go    # Data sources registry CRUD
│   │   ├── cycles.go          # Reset cycle CRUD (Phase 7)
│   │   ├── claude_snapshots.go # Claude Code rate limit snapshots (Phase 9)
│   │   ├── codex_snapshots.go # Codex/ChatGPT usage snapshots (Phase 11)
│   │   ├── sessions.go        # Usage session CRUD (Phase 11)
│   │   ├── usage_logs.go      # Manual usage log CRUD (Phase 11)
│   │   └── import.go          # JSON import with merge/dedup (Phase 11)
│   │
│   ├── readiness/             # Pure computation engine (zero I/O)
│   │   └── readiness.go       # Calculate readiness from snapshots
│   │
│   └── web/                   # HTTP server + embedded dashboard
│       ├── server.go          # 27 REST handlers, agent management
│       ├── handlers_subscriptions.go  # Subscription CRUD, overview, presets, CSV
│       └── static/            # Embedded via Go embed.FS
│           ├── index.html     # 4-tab dashboard
│           ├── style.css      # Design system (~2500 lines)
│           ├── app.js         # Dashboard logic (~2500 lines)
│           └── manifest.json  # PWA manifest
│
├── docs/
│   ├── VISION.md, ARCHITECTURE.md, API_SPEC.md
│   ├── DATA_MODEL.md, TESTING.md
│
├── go.mod / go.sum
├── .gitignore
└── README.md
```

## Dependencies

| Dependency | Purpose |
|-----------|---------|
| `modernc.org/sqlite` | Pure-Go SQLite (no CGo → true single binary) |
| `github.com/modelcontextprotocol/go-sdk` | MCP server for AI agent integration |
| Go stdlib | Everything else (HTTP, JSON, embed, crypto, os) |

**That's it.** No web frameworks, no ORMs, no logging libraries, no npm.

## MCP Server (AI Agent Integration)

Niyantra exposes quota intelligence to AI coding agents via the [Model Context Protocol](https://modelcontextprotocol.io).

**8 tools available:**

| Tool | What it does |
|------|--------------|
| `quota_status` | All accounts with per-group readiness |
| `model_availability` | Check a specific model by name |
| `usage_intelligence` | Consumption rates and projections |
| `budget_forecast` | Burn rate and monthly projection |
| `best_model` | Recommend least-exhausted model |
| `analyze_spending` | Spending analysis with savings detection |
| `switch_recommendation` | Cross-account routing recommendation |
| `codex_status` | Codex/ChatGPT detection and quota status |

**Setup** — add to Claude Desktop's `claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "niyantra": {
      "command": "path/to/niyantra.exe",
      "args": ["mcp"]
    }
  }
}
```

Then ask: *"What's my Windsurf quota?"* or *"Which model should I use?"* or *"What's my Codex status?"*

## Stats

- **~13,000 lines** of code (Go + JS + CSS + HTML)
- **~18 MB** compiled binary (includes embedded SQLite engine + Chart.js loaded from CDN)
- **0** external runtime dependencies
- **27 REST endpoints + 8 MCP tools**
- **200+ manual test cases**
- **PWA installable** — add to home screen / install as app

## License

Private. Not for redistribution.
