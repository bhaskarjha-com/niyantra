# Niyantra

**Local-first AI subscription & quota dashboard.**

Niyantra (Sanskrit: नियन्त्र, "controller") is a single-binary dashboard that combines **automated quota detection** with **manual subscription tracking** for all your AI tools. It auto-captures Antigravity quotas from the local language server and lets you manually track spending, limits, and renewals for every AI service you use — ChatGPT, Claude, Cursor, Copilot, and 26+ others.

> **Zero daemon. One API call. All local.**

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
- **Smart insights** — trial warnings, top spending category, renewal alerts, annual savings potential
- **Monthly/annual spend** with category breakdown
- **Upcoming renewals** with countdown
- **Quick Links Hub** — clickable grid of all your AI tool dashboards
- **Ready Now advisor** — shows which auto-tracked tools have quota right now
- **CSV Export** — download all subscriptions for tax/expense reports

### Settings Tab

- **Budget** — set monthly AI spending threshold with alerts
- **Display** — default currency (USD/EUR/GBP/INR/CAD/AUD), theme (dark/light/system)
- **Data** — CSV export, database location
- **Keyboard shortcuts** — reference grid
- **About** — version info, schema version, preset count

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `1` `2` `3` `4` | Switch tabs |
| `N` | New subscription |
| `S` | Snap quota |
| `/` | Search subscriptions |
| `Esc` | Close modal |

### What It Does NOT Show

- **AI Credits** — not available via local language server API. Requires cloud API that the LS doesn't expose.

## CLI Commands

| Command | What it does | Network calls |
|---------|-------------|---------------|
| `niyantra snap` | Capture current account's quota | 1 (local LS) |
| `niyantra status` | Show all accounts' readiness | 0 |
| `niyantra serve` | Start web dashboard | 0 (+ 1 per snap) |
| `niyantra version` | Print version | 0 |

### Flags

```
--port     Dashboard port (default: 9222)
--db       Database path (default: ~/.niyantra/niyantra.db)
--auth     HTTP basic auth for dashboard (user:pass)
--debug    Enable verbose logging
```

## Design Principles

1. **Zero daemon** — No background process, no polling, no system tray. You invoke it when you need it.
2. **One API call per snap** — Minimal footprint on the language server. No pre-flight checks, no token refresh.
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
│   └── main.go                # snap, status, serve, version commands
│
├── internal/
│   ├── client/                # Antigravity language server client
│   │   ├── client.go          # Detect + FetchQuotas (1 API call)
│   │   ├── detect_windows.go  # Windows: CIM → PowerShell → WMIC
│   │   ├── detect_unix.go     # macOS/Linux: ps aux
│   │   ├── ports.go           # Port discovery (lsof/ss/netstat)
│   │   ├── probe.go           # Connect RPC endpoint validation
│   │   ├── types.go           # API response structs
│   │   └── helpers.go         # Model grouping logic
│   │
│   ├── store/                 # SQLite persistence
│   │   ├── store.go           # Open, migrate (v1→v2), close
│   │   ├── snapshots.go       # Insert, LatestPerAccount, History
│   │   ├── accounts.go        # GetOrCreateAccount (upsert by email)
│   │   ├── subscriptions.go   # Subscription CRUD, overview stats, renewals
│   │   └── presets.go         # 26 platform preset templates
│   │
│   ├── readiness/             # Pure computation engine (zero I/O)
│   │   └── readiness.go       # Calculate readiness from snapshots
│   │
│   └── web/                   # HTTP server + embedded dashboard
│       ├── server.go          # Server setup, quota handlers, auto-link
│       ├── handlers_subscriptions.go  # Subscription CRUD, overview, presets, CSV
│       └── static/            # Embedded via Go embed.FS
│           ├── index.html     # 4-tab dashboard (Quotas/Subs/Overview/Settings)
│           ├── style.css      # Design system (charts, modals, tabs, themes)
│           ├── app.js         # Dashboard logic (CRUD, charts, search, shortcuts)
│           └── manifest.json  # PWA manifest for installability
│
├── docs/                      # Documentation
│   ├── VISION.md              # Why this tool exists
│   ├── ARCHITECTURE.md        # System design + component details
│   ├── API_SPEC.md            # REST API reference (11 endpoints)
│   ├── DATA_MODEL.md          # SQLite schema v2 + queries
│   └── TESTING.md             # Manual testing guide (80+ test cases)
│
├── go.mod                     # Single dependency: modernc.org/sqlite
├── go.sum
├── .gitignore
└── README.md
```

## Dependencies

| Dependency | Purpose |
|-----------|---------|
| `modernc.org/sqlite` | Pure-Go SQLite (no CGo → true single binary) |
| Go stdlib | Everything else (HTTP, JSON, embed, crypto, os) |

**That's it.** No web frameworks, no ORMs, no logging libraries, no npm.

## Stats

- **~3,500 lines** of code (Go + JS + CSS + HTML)
- **~16 MB** compiled binary (includes embedded SQLite engine + Chart.js loaded from CDN)
- **0** external runtime dependencies
- **PWA installable** — add to home screen / install as app

## License

Private. Not for redistribution.
