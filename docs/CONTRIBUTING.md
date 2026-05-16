# Contributing to Niyantra

## Quick Start

```bash
# Clone
git clone https://github.com/bhaskarjha-com/niyantra.git
cd niyantra

# Build (macOS/Linux)
make build

# Build (Windows or without make)
go build -o niyantra.exe ./cmd/niyantra

# Verify
./niyantra version

# Seed sample data + explore (macOS/Linux)
make demo

# Seed sample data + explore (Windows)
go run ./cmd/niyantra demo
go run ./cmd/niyantra serve
```

### Requirements

- **Go 1.25+** — the only build dependency for the backend
- **Node.js 18+** — for TypeScript type-checking and esbuild bundling (frontend only)
- No CGo, no C compiler needed (`modernc.org/sqlite` is pure Go)

## Run Locally

```bash
# Start the dashboard with sample data
make demo          # macOS/Linux
# OR
go run ./cmd/niyantra demo
go run ./cmd/niyantra serve  # Windows

# Or with real data (Antigravity must be running)
./niyantra snap
./niyantra serve   # http://localhost:9222
```

## Make Targets

| Target | What it does |
|--------|-------------|
| `make build` | Build binary with version injection |
| `make run` | Build + launch dashboard |
| `make demo` | Seed sample data + launch dashboard |
| `make test` | Run all tests with race detection |
| `make vet` | Run Go vet |
| `make js` | Bundle frontend TypeScript → `app.js` (dev) |
| `make js-prod` | Bundle + minify for production |
| `make js-watch` | Watch mode — auto-rebuild on save |
| `make css` | Bundle CSS source files → `style.css` (dev) |
| `make css-prod` | Bundle + minify CSS for production |
| `make css-watch` | Watch mode — auto-rebuild CSS on save |
| `make clean` | Remove built binaries |

## Project Layout

```
cmd/niyantra/main.go              ← CLI entrypoint (snap, status, serve, mcp, demo, backup, restore)

internal/
  client/                          ← Antigravity language server detection + API call
    client.go                         Detect() + FetchQuotas() — the only external call
    detect_windows.go                 Windows: CIM → PowerShell → WMIC fallback chain
    detect_unix.go                    macOS/Linux: ps aux
    ports.go                          Port discovery via lsof/ss/netstat
    probe.go                          Connect RPC endpoint validation
    types.go                          API response structs
    helpers.go                        Model grouping logic (claude_gpt / gemini_pro / gemini_flash)

  store/                           ← SQLite persistence (schema v18, 18 tables, 23 Go files)
    store.go                          Open, migrate schema (v1→v18), close
    snapshots.go                      InsertSnapshot, LatestPerAccount, History
    accounts.go                       GetOrCreateAccount (upsert by email)
    subscriptions.go                  Subscription CRUD, 26 presets
    config.go                         Typed key-value config
    activity.go                       Activity log CRUD
    codex.go                          Codex snapshot persistence
    sessions.go                       Usage session tracking
    usage_logs.go                     Manual usage log CRUD
    cursor.go                         Cursor snapshot persistence
    gemini.go                         Gemini snapshot persistence
    copilot.go                        Copilot snapshot persistence
    webpush.go                        WebPush subscription CRUD
    heatmap.go                        Activity heatmap data queries
    token_usage.go                    Claude token usage daily storage

  readiness/                       ← Pure readiness computation (zero I/O)
    readiness.go                      Calculate() — groups models, computes % + countdowns
    readiness_test.go                 9 unit tests

  advisor/                         ← Switch advisor engine
    advisor.go                        Multi-factor scoring (remaining%, burn rate, reset time)
    advisor_test.go                   7 unit tests

  agent/                           ← Auto-capture polling agent
    agent.go                          Ticker loop, exponential backoff, graceful shutdown
  tracker/                         ← Usage intelligence
    tracker.go                        Reset cycle detection, consumption rates, exhaustion forecast

  codex/                           ← Codex/ChatGPT integration
    codex.go                          OAuth auth detection + API polling

  claude/                           ← Claude Code provider
    deep.go                           JSONL session parser (token usage, model normalization)
    bridge.go                         Statusline settings patch + rate limit monitoring

  cursor/                           ← Cursor provider
    cursor.go                         Session token auth + HTTP API polling

  gemini/                           ← Gemini CLI provider
    gemini.go                         OAuth discovery + GCP API (loadCodeAssist + retrieveUserQuota)

  copilot/                          ← GitHub Copilot provider
    copilot.go                        PAT auth + GitHub billing API

  notify/                          ← Quad-channel notification engine
    engine.go                         Threshold check, guard, OnNotify callback, quad-channel dispatch
    notify.go                         Cross-platform OS notifications (PowerShell/osascript/notify-send)
    smtp.go                           Pure Go SMTP (plain/STARTTLS/TLS), HTML templates
    webhook.go                        Multi-service webhooks (Discord/Telegram/Slack/Generic)
    webpush.go                        VAPID + RFC 8291 encryption (zero x/crypto)

  forecast/                        ← Forecasting engine
    forecast.go                       TTX + cost forecasting

  costtrack/                       ← Cost tracking engine
    costtrack.go                      Blended model pricing + cost calculation

  tokenusage/                      ← Token usage analytics
    tokenusage.go                     Claude JSONL → daily aggregation pipeline

  gitcorr/                         ← Git commit cost correlation
    gitcorr.go                        Git log ↔ Claude session timestamp correlation

  mcpserver/                       ← MCP stdio + Streamable HTTP server
    server.go                         11 tools: quota, models, usage, budget, best_model, spending, switch, codex, forecast, token_usage, git_commit_costs

  web/                             ← Modular HTTP server (17 Go files)
    server.go                         Server struct, lifecycle, route table
    middleware.go                     Auth + CORS middleware
    helpers.go                        JSON response utilities
    compute.go                        Forecast/cost engines (pure logic, no HTTP)
    handlers_quota.go                 status, snap, history, usage endpoints
    handlers_config.go                config, activity, mode, onConfigChanged
    handlers_ops.go                   healthz, Claude, backup, notify, export, alerts, advisor, webpush
    handlers_codex.go                 Codex, sessions, usage logs endpoints
    handlers_data.go                  accounts, snapshots, pricing endpoints
    handlers_forecast.go              cost + TTX forecast endpoints
    handlers_subscriptions.go         Subscription CRUD, overview, presets, CSV
    handlers_cursor.go                Cursor status/snap endpoints
    handlers_gemini.go                Gemini status/snap endpoints
    handlers_copilot.go               Copilot status/snap endpoints
    handlers_heatmap.go               Activity heatmap data
    static/                           Embedded via Go embed.FS
      index.html                       Single-page dashboard shell
      style.css                        Design system (CSS variables, dark/light themes)
      app.js                           GENERATED — do not edit (bundled from src/)
      sw.js                            Service Worker for WebPush notifications
    src/                              TypeScript source (30 modules, strict mode)
      main.ts                          Entry point: imports + DOMContentLoaded init
      subscriptions.ts                 Subscription cards, modal, search
      core/                            state.ts, utils.ts, api.ts, theme.ts
      quotas/                          render.ts, expand.ts, features.ts
      overview/                        overview.ts, budget.ts, insights.ts, cost.ts, calendar.ts, gitCosts.ts
      charts/                          history.ts (Chart.js integration)
      settings/                        settings.ts, pricing.ts, mode.ts, activity.ts, data.ts
      advanced/                        snap.ts, palette.ts, keyboard.ts, alerts.ts, codex.ts, claude.ts, heatmap.ts, tokenUsage.ts
      types/                           api.ts (API response interfaces)
```

### Frontend Build Pipeline

```
TypeScript sources (30 .ts files, strict mode)
        ↓ esbuild --bundle --format=iife --minify
    static/app.js (~119 KB, single IIFE)
        ↓ go:embed
    Go binary (self-contained)
```

## Key Design Decisions (read these first!)

### 1. No `hidden` attribute for toggles — use CSS classes

The HTML `hidden` attribute sets `display: none`, but any CSS rule with `display: flex/grid` silently overrides it. Use `.is-expanded` / `.is-collapsed` CSS classes instead.

```css
/* ✅ Correct */
.model-details { display: none; }
.model-details.is-expanded { display: flex; }

/* ❌ Wrong — display:flex overrides [hidden] */
.model-details { display: flex; }  /* hidden attr ignored! */
```

### 2. Serialize durations as seconds, not Go nanoseconds

Go's `time.Duration` marshals to JSON as **nanoseconds** (int64). Always convert to seconds at the Go layer:

```go
// ✅ Correct
TimeUntilResetSec float64 `json:"timeUntilResetSec"`

// ❌ Wrong — JS receives nanoseconds like 16200000000000
TimeUntilReset time.Duration `json:"timeUntilReset"`
```

### 3. LatestPerAccount uses MAX(id), not MAX(captured_at)

If two snapshots have the same timestamp (same-second rapid clicks), `MAX(captured_at)` returns both. `MAX(id)` is always unique.

### 4. Zero-daemon by default

Auto-capture only runs when explicitly enabled AND `niyantra serve` is running. It uses a ticker loop with configurable interval (30s-300s) and exponential backoff on failures. There is no standalone background daemon.

### 5. Event delegation for dynamic content

Since `renderAccounts()` rebuilds `innerHTML`, inline `onclick` handlers can fail. Use event delegation on the grid container:

```js
grid.addEventListener('click', function(e) {
  var row = e.target.closest('.account-row[data-toggle]');
  if (!row) return;
  // ... toggle logic
});
```

### 6. Provenance on every data point

Every snapshot must carry `capture_method` (manual/auto), `capture_source` (cli/ui/server), and `source_id` (antigravity/codex/claude). This is checked in code review.

## Testing

```bash
# Run all tests
make test

# Run specific packages
go test -count=1 ./internal/readiness/
go test -count=1 ./internal/advisor/

# Build + vet
make build
make vet
```

Current coverage: 148 tests across 13 files in 10 packages (`advisor`, `claude`, `costtrack`, `forecast`, `mcpserver`, `notify`, `readiness`, `store`, `tracker`, `web`).

## Common Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| "language server process not found" | Antigravity IDE not running | Start the IDE with a project open |
| "not authenticated" | No account logged in | Log into Antigravity in the IDE |
| Dashboard shows stale data | No auto-refresh by design | Click "Snap Now" or reload page |
| "AI Credits" missing | Not in local LS API | Cannot be fixed — cloud-only data |
| Binary won't cross-compile | `modernc.org/sqlite` needs correct GOOS/GOARCH | Set env vars explicitly |
| Demo data won't seed | Database already has data | Use a fresh db: `niyantra demo --db /tmp/demo.db` |
