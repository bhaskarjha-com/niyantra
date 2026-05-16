# Testing Guide: Niyantra

> **Updated:** v0.26.0 · Schema v18 · 148 tests across 13 files in 10 packages

## Automated Test Suite

### Running Tests

```bash
# Full suite (all 148 tests)
go test ./... -count=1 -v

# With race detection
go test ./... -race -count=1

# Specific package
go test -count=1 -v ./internal/notify/

# Build + vet (CI gate)
go build ./...
go vet ./...
```

### Test Coverage by Package

| # | Package | File | Tests | What's Covered |
|---|---------|------|-------|----------------|
| 1 | `advisor` | `advisor_test.go` | 8 | Switch/stay/wait decisions, stale data, multi-account scoring |
| 2 | `claude` | `deep_test.go` | 7 | JSONL parsing, model normalization, date extraction, token aggregation |
| 3 | `costtrack` | `costtrack_test.go` | 14 | Blended pricing, group cost, account aggregate, currency formatting |
| 4 | `forecast` | `forecast_test.go` | 10 | TTX forecasting, burn rate, trend calculation, edge cases |
| 5 | `mcpserver` | `mcpserver_test.go` | 2 | Tool registration, response formatting |
| 6 | `notify` | `engine_test.go` | 8 | Guard, OnReset, OnNotify callback, disabled/threshold skip |
| 7 | `notify` | `smtp_test.go` | 8 | Config validation, recipients, message build, HTML templates |
| 8 | `notify` | `webhook_test.go` | 12 | Discord/Slack/Generic delivery via httptest, severity, escaping |
| 9 | `notify` | `webpush_test.go` | 14 | VAPID key gen, HKDF, base64 decode, full send via httptest |
| 10 | `readiness` | `readiness_test.go` | 18 | Threshold, grouping, staleness, reset inference, edge cases |
| 11 | `store` | `store_test.go` | 20 | Migration v1→v18, config CRUD, retention, Quick Adjust, heatmap |
| 12 | `tracker` | `tracker_test.go` | 2 | Multi-account contamination, concurrent safety |
| 13 | `web` | `server_test.go` | 7 | CORS, Content-Type, body limits, localhost binding, preflight |
| | **TOTAL** | **13 files** | **148** | |

### Test Patterns

All tests use Go's standard `testing` package — no third-party test frameworks.

- **httptest**: Used in `smtp_test.go`, `webhook_test.go`, `webpush_test.go`, `server_test.go` to simulate HTTP servers
- **In-memory SQLite**: `store_test.go` creates fresh databases per test
- **Table-driven tests**: Used extensively in `readiness_test.go`, `costtrack_test.go`, `forecast_test.go`
- **Crypto validation**: `webpush_test.go` generates real P-256 keys and validates VAPID JWT structure

---

## Manual Testing Guide

Complete feature checklist for manual verification. Each section has step-by-step test cases with expected results.

**Prerequisites:**
1. Build: `go build -o niyantra.exe ./cmd/niyantra`
2. Run: `.\niyantra.exe serve --debug`
3. Open: `http://localhost:9222`

---

## 1. Dashboard Shell

### 1.1 Page Load
- [ ] Dashboard loads without blank screen or errors
- [ ] Title shows "Niyantra" in browser tab
- [ ] Inter font loads (text should NOT be Times New Roman / serif)
- [ ] Default tab is **Quotas** (highlighted in nav)
- [ ] Dark theme applied by default

### 1.2 Tab Navigation
- [ ] Click **Subscriptions** tab → panel switches, tab highlights
- [ ] Click **Overview** tab → panel switches, tab highlights
- [ ] Click **Settings** tab → panel switches, tab highlights
- [ ] Click **Quotas** tab → returns to quota grid
- [ ] Only one tab is highlighted at a time

### 1.3 Theme Toggle
- [ ] Click moon/sun icon in header → switches to light theme
- [ ] All cards, modals, inputs update colors correctly
- [ ] Click again → switches back to dark
- [ ] Refresh page → theme persists (localStorage)

---

## 2. Quotas Tab (Auto-Tracked)

### 2.1 Provider-Sectioned Layout
- [ ] Accounts grouped by provider: Antigravity, Codex/ChatGPT, Claude Code, Cursor, Gemini CLI, Copilot
- [ ] Each section has collapsible header with provider name and color
- [ ] Click section header → collapses/expands the section
- [ ] **Provider filter dropdown**: All / Antigravity / Codex / Claude / Cursor / Gemini / Copilot
- [ ] **Status filter**: Ready / Low / Empty
- [ ] Filters combine: provider + status work simultaneously
- [ ] **Tag filter**: filter accounts by tag (work, personal, etc.)

### 2.2 Snap
- [ ] Click **Snap Now** button (primary action on split-button)
- [ ] If Antigravity is running: toast shows "✅ Snapshot captured" with email
- [ ] If NOT running: toast shows error "Could not detect..." (red)
- [ ] Click **▾** dropdown → "Snap All Sources" captures all enabled providers

### 2.3 Account Cards
- [ ] Row shows: email, plan badge, staleness ("2m ago"), status badge
- [ ] Status badge: "Ready" (green), "Low" (yellow), "Exhausted" (red)
- [ ] Credit renewal countdown badge (if renewal day configured)
- [ ] Quota cells show color-coded progress bars
- [ ] Click row → expands to show per-model breakdown with progress bars
- [ ] Account notes display inline (if set)
- [ ] Tag pills render with correct colors

### 2.4 Quick Adjust — Group Level
- [ ] Hover over quota group cell → **−5** and **+5** buttons appear
- [ ] Click **−5** → percentage decreases by 5, bar shrinks, toast confirms
- [ ] Click **+5** → percentage increases by 5, bar grows
- [ ] Refresh page → adjusted values persist (stored in DB)

### 2.5 Quick Adjust — Model Level
- [ ] Expand an account → hover over model row
- [ ] **−10**, **−5**, **+5**, **+10** buttons appear
- [ ] Only the specific model is affected (other models unchanged)
- [ ] Group-level aggregate recalculates after model adjustment

### 2.6 Quota History Chart
- [ ] Chart section appears below the account grid
- [ ] Account selector dropdown: "All Accounts" + one option per tracked account
- [ ] Range selector: "Last 20" / "Last 50" / "Last 100"
- [ ] Chart.js line chart with color-coded lines per quota group
- [ ] Hover tooltip showing exact percentage
- [ ] Chart adapts to theme (dark/light)

### 2.7 Activity Heatmap
- [ ] GitHub-style 365-day contribution grid
- [ ] Color intensity reflects daily snapshot count
- [ ] Tooltip shows date and count on hover
- [ ] Configurable lookback via `heatmap_lookback_days` config key

---

## 3. Subscriptions Tab

### 3.1 Add Subscription (Preset)
- [ ] Click **+ Add** → modal opens with platform datalist
- [ ] Type platform name → 26 presets auto-fill (cost, category, notes, URLs)
- [ ] Click **Save** → toast confirms, card appears in grid

### 3.2 Add Subscription (Custom)
- [ ] Enter custom platform, category, cost, currency, cycle
- [ ] Card appears with correct formatting

### 3.3 Edit / Delete Subscription
- [ ] **Edit** → modal opens with pre-filled values
- [ ] **Delete** → confirmation dialog → card removed

### 3.4 Filters & Search
- [ ] Status filter, category filter, text search all work
- [ ] Both filters work simultaneously

### 3.5 CSV Export
- [ ] Click **📥 Export CSV** → downloads file with all subscription data

---

## 4. Overview Tab

### 4.1 Financial Summary
- [ ] Monthly spend = sum of active subscriptions (monthly-normalized)
- [ ] Annual estimate = monthly × 12
- [ ] Category breakdown sorted by highest spend

### 4.2 Budget
- [ ] Set budget → alert bar shows green/yellow/red based on utilization
- [ ] Budget Forecast card: burn rate, projected spend, on-track/over status

### 4.3 Switch Advisor
- [ ] Shows recommended account with switch/stay/wait action
- [ ] Multi-factor score: 60% remaining, 20% burn rate, 20% reset time

### 4.4 Smart Insights
- [ ] Colored chips for active subs, trials, renewals, anomalies, savings

### 4.5 Renewal Calendar
- [ ] CSS grid month-view with pin markers on renewal dates
- [ ] Month navigation (prev/next)

### 4.6 Provider Health Cards
- [ ] Per-provider summary: Antigravity, Codex, Claude, Cursor, Gemini, Copilot

### 4.7 Estimated Cost Tracking
- [ ] Per-model estimated spend based on quota delta × model pricing
- [ ] KPI cards: daily cost, monthly projected, top model

### 4.8 Git Commit Costs
- [ ] Commit-level cost attribution (±30 min session correlation)
- [ ] Branch-level aggregation
- [ ] Sparkline visualization

---

## 5. Settings Tab

### 5.1 Capture & Sources
- [ ] Auto Capture toggle (OFF by default) → mode badge changes
- [ ] Poll Interval appears when ON
- [ ] Data Sources: Antigravity, Claude Code, Codex, Cursor, Gemini, Copilot
- [ ] Each source shows capture count and last capture time

### 5.2 Provider Settings
- [ ] Claude Code Bridge toggle + status indicator
- [ ] Codex Capture toggle
- [ ] Cursor Capture toggle
- [ ] Gemini CLI Capture toggle
- [ ] Copilot toggle + PAT input (masked in API)

### 5.3 Notifications — OS Native
- [ ] Toggle off (default) → threshold hidden
- [ ] Toggle on → threshold input (5-50%) and test button appear
- [ ] Click **🔔 Test** → OS notification fires

### 5.4 Notifications — SMTP Email (F11)
- [ ] SMTP toggle reveals 7 config fields (host, port, user, pass, from, to, tls)
- [ ] `smtp_pass` shows "configured" (never reveals actual value)
- [ ] Click **📧 Send Test** → test email sent → toast confirms

### 5.5 Notifications — Webhook (F22)
- [ ] Webhook toggle reveals service type, URL, secret fields
- [ ] Service type dropdown: Discord / Telegram / Slack / Generic
- [ ] Labels dynamically change per service type
- [ ] `webhook_secret` shows "configured" (masked)
- [ ] Click **🔗 Send Test** → test webhook fires

### 5.6 Notifications — WebPush (F19)
- [ ] WebPush toggle reveals subscription controls
- [ ] **Subscription Status** badge: 🟢 Subscribed / ⚪ Not subscribed
- [ ] Click **🔔 Subscribe** → browser permission prompt → Service Worker registered
- [ ] After subscribe: badge → 🟢, button → 🔕 Unsubscribe
- [ ] Click **🔔 Send Test** → browser push notification appears
- [ ] Click **🔕 Unsubscribe** → subscription removed
- [ ] Graceful degradation if browser doesn't support PushManager

### 5.7 Budget & Display
- [ ] Monthly Budget input → persists across refreshes
- [ ] Currency selector: USD, EUR, GBP, INR, CAD, AUD
- [ ] Theme: Dark, Light, System

### 5.8 Data Management
- [ ] Snapshot Retention input (default 365)
- [ ] Database location display
- [ ] 💾 Backup download button

### 5.9 Activity Log
- [ ] Shows recent events with color-coded badges
- [ ] Filters: All Events, Snaps, Failed Snaps, Config Changes, Quota Alerts, Server Start
- [ ] ↻ Refresh button
- [ ] Event types: snap (blue), snap_failed (red), config_change (purple), quota_alert (amber), server_start (green)

### 5.10 Model Pricing Config
- [ ] Per-model $/1M token pricing editor
- [ ] Preloaded defaults for common models

### 5.11 About Section
- [ ] Shows "Schema v18 · 26 presets · Mode: Manual/Auto · X active sources"

---

## 6. Keyboard Shortcuts

- [ ] `1` → Quotas, `2` → Subscriptions, `3` → Overview, `4` → Settings
- [ ] `N` → new subscription modal
- [ ] `Esc` → close any modal
- [ ] `S` → trigger snap
- [ ] `/` → focus subscription search
- [ ] `Ctrl+K` → command palette

### Shortcut Safety
- [ ] Do NOT fire when typing in an input field
- [ ] Do NOT fire when modal is open (except Esc)

---

## 7. Command Palette

- [ ] `Ctrl+K` → opens with blurred overlay
- [ ] Fuzzy search filters commands
- [ ] Arrow key navigation + Enter to execute
- [ ] 12+ commands: Snap, tabs, New Subscription, Toggle Auto-Capture, Export, Backup, Theme

---

## 8. CLI Commands

### 8.1 Core Commands
```
niyantra snap          # Capture Antigravity quota
niyantra status        # Show readiness (0 network calls)
niyantra serve         # Start dashboard + API
niyantra serve --port 8080 --debug --auth user:pass
niyantra mcp           # Start MCP stdio server
niyantra version       # Print version string
```

### 8.2 Data Commands
```
niyantra demo          # Seed sample data
niyantra backup        # Create timestamped backup
niyantra restore <file>  # Restore from backup
```

---

## 9. MCP Server (11 tools)

### 9.1 Stdio Transport
- [ ] `niyantra mcp` starts without error
- [ ] JSON-RPC initialize → response with `serverInfo.name: "niyantra"`
- [ ] `tools/list` returns 11 tools

### 9.2 Streamable HTTP Transport
- [ ] `POST /mcp` endpoint accepts MCP protocol messages
- [ ] SSE streaming for long-running tool calls
- [ ] Session management via `Mcp-Session-Id` header

### 9.3 Tool Inventory
| Tool | Description |
|------|-------------|
| `quota_status` | All accounts with readiness state |
| `model_availability` | Check specific model availability |
| `usage_intelligence` | Rate, projection, cycle data per model |
| `budget_forecast` | Burn rate, projected spend, on-track |
| `best_model` | Recommend best available model in group |
| `analyze_spending` | Spending analysis, savings detection |
| `switch_recommendation` | AI-powered switch advice |
| `codex_status` | Codex/ChatGPT quota state |
| `quota_forecast` | TTX forecasting per model |
| `token_usage` | Claude Code token analytics |
| `git_commit_costs` | Git commit cost correlation |

---

## 10. Notifications — Quad-Channel

### 10.1 OS Native
- [ ] Set threshold to 50%, snap when below → OS notification fires
- [ ] Snap again → no duplicate (once-per-cycle guard)
- [ ] After model resets → notification can fire again

### 10.2 SMTP Email (F11)
- [ ] Configure SMTP → test email arrives
- [ ] Quota alert triggers HTML email with model/percentage

### 10.3 Webhook (F22)
- [ ] Configure Discord/Telegram/Slack/Generic → test fires
- [ ] Quota alert sends service-specific payload

### 10.4 WebPush (F19)
- [ ] Subscribe browser → test push appears
- [ ] Quota alert sends push to all subscribed browsers
- [ ] Works even when dashboard tab is closed

### 10.5 System Alert + Activity Log
- [ ] Notification fires → alert banner appears at top
- [ ] Alert has dismiss button → removed on click
- [ ] Activity log shows `quota_alert` event

---

## 11. Provider-Specific Testing

### 11.1 Antigravity
- [ ] Detects running LS process (CIM/ps)
- [ ] Fetches quota via Connect RPC
- [ ] Stores AI Credits (availableCredits, promptCredits, flowCredits)

### 11.2 Codex / ChatGPT
- [ ] Detects `~/.codex/auth.json` credentials
- [ ] OAuth token refresh works
- [ ] 5h, 7d, code review quotas tracked
- [ ] OIDC identity: name + avatar from JWT

### 11.3 Claude Code
- [ ] Statusline bridge patches `~/.claude/settings.json`
- [ ] Rate limit data captured (5h/7d meters)
- [ ] Deep tracking: JSONL sessions parsed for token usage
- [ ] Token analytics: per-day input/output/cache costs

### 11.4 Cursor
- [ ] Detects session token from filesystem
- [ ] Polls `cursor.com/api/usage` endpoint
- [ ] Request count + USD credit tracking

### 11.5 Gemini CLI
- [ ] Detects OAuth credentials from `~/.config/gemini/`
- [ ] Polls GCP billing/quota APIs
- [ ] Rate limit tracking

### 11.6 GitHub Copilot
- [ ] Accepts GitHub PAT in settings (masked in API)
- [ ] Polls GitHub billing endpoint
- [ ] Usage metrics tracked

---

## 12. Data Integrity

### 12.1 Database Migration
- [ ] Delete `~/.niyantra/niyantra.db` → restart → v18 schema created from scratch
- [ ] `PRAGMA user_version` returns `18`
- [ ] All 18 tables exist

### 12.2 Config Masking
- [ ] `GET /api/config` returns `"configured"` for:
  - `copilot_pat`
  - `smtp_pass`
  - `webhook_secret`
  - `webpush_vapid_private`

### 12.3 Provenance
- [ ] Snap via UI → activity log shows `snap`, `manual via ui`
- [ ] Snap via CLI → activity log shows `snap`, `manual via cli`
- [ ] Auto snap → activity log shows `snap`, `auto via server`

### 12.4 Concurrent Access
- [ ] Open dashboard in 2 tabs → data consistent after snap in either

---

## 13. Auto-Capture Agent

### 13.1 Enable/Disable
- [ ] Settings → toggle ON → toast + mode badge changes to green "AUTO"
- [ ] Toggle OFF → mode badge returns to blue "MANUAL"

### 13.2 Polling
- [ ] Set interval, enable → snapshots auto-captured
- [ ] Activity log shows auto-captured events
- [ ] Live poll interval reload (change interval while running)

### 13.3 Backoff
- [ ] 3 consecutive failures → polling pauses
- [ ] LS starts → agent retries and resumes

### 13.4 Graceful Shutdown
- [ ] Ctrl+C → clean shutdown, no orphaned goroutines

---

## 14. Backup & Restore

- [ ] `niyantra backup` → creates timestamped `.db.bak` file
- [ ] `niyantra restore <file>` → prompts for confirmation → database replaced
- [ ] Dashboard backup button → downloads valid SQLite file
- [ ] `GET /api/export/json` → full JSON export (all tables)
- [ ] `POST /api/import/json` → additive merge with deduplication

---

## 15. Security

- [ ] `--auth user:pass` → dashboard requires HTTP basic auth
- [ ] Unauthenticated request → 401
- [ ] CORS headers set correctly (X-Content-Type-Options, CSP, XSS-Protection)
- [ ] Environment variables work: NIYANTRA_PORT, NIYANTRA_BIND, NIYANTRA_DB, NIYANTRA_AUTH
- [ ] `/healthz` returns 200 with version and uptime (no auth required)

---

## 16. Docker

- [ ] `docker build -t niyantra .` → builds successfully
- [ ] `docker compose up` → starts on port 9222
- [ ] Database persists in mounted volume
- [ ] Multi-arch support: linux/amd64, linux/arm64

---

## Test Results Template

| Section | Pass | Fail | Notes |
|---------|------|------|-------|
| 1. Dashboard Shell | | | |
| 2. Quotas Tab | | | |
| 3. Subscriptions Tab | | | |
| 4. Overview Tab | | | |
| 5. Settings Tab | | | |
| 6. Keyboard Shortcuts | | | |
| 7. Command Palette | | | |
| 8. CLI Commands | | | |
| 9. MCP Server | | | |
| 10. Notifications (4 channels) | | | |
| 11. Provider Testing (7) | | | |
| 12. Data Integrity | | | |
| 13. Auto-Capture Agent | | | |
| 14. Backup & Restore | | | |
| 15. Security | | | |
| 16. Docker | | | |

**Tester:** _______________  
**Date:** _______________  
**Build:** `niyantra.exe` v0.26.0 from commit _______________
