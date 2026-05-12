# Architecture: Niyantra

## System Overview

```
User Interfaces
  CLI (snap/status/serve/mcp/demo/backup)
  Dashboard (4 tabs, embedded web app, PWA)
  MCP Server (8 tools, AI agent access)
          |
Application Layer
  agent/        - polling loop + session management
  client/       - LS detection + quota fetch (Connect RPC)
  codex/        - OAuth + Codex API polling + OIDC JWT parsing
  claudebridge/ - statusline patch + read
  advisor/      - switch recommendation engine
  tracker/      - cycle detection + intelligence
  readiness/    - pure readiness computation (reset-time-corrected)
  notify/       - OS-native notifications
          |
Storage Layer
  store/  - SQLite v9 (11 tables)
  config  - typed key-value settings
  Pure Go: modernc.org/sqlite (no CGo)
```

## Package Dependency Graph

```
cmd/niyantra/main.go
  +-- client     (detect LS, fetch quotas)
  +-- store      (SQLite, all persistence)
  +-- web        (HTTP server + dashboard)
  |    +-- agent      (polling loop)
  |    +-- tracker    (cycles + sessions)
  |    +-- advisor    (switch engine)
  |    +-- codex      (ChatGPT integration)
  |    +-- claudebridge (Claude Code bridge)
  |    +-- notify     (OS-native notifications + alert wiring)
  +-- mcpserver  (MCP tools over stdio)
  |    +-- store, tracker, advisor, codex, readiness
  +-- readiness  (pure computation, zero I/O)
```

## 1. internal/client/ - Antigravity Language Server Client

Detects the running Antigravity language server and fetches quota data via Connect RPC.

Detection strategy (platform-specific):

| Platform | Method | Fallback 1 | Fallback 2 |
|----------|--------|-----------|-----------|
| Windows | CIM (Win32_Process with -Filter) | Get-Process -match | WMIC |
| macOS/Linux | ps aux grep | - | - |

Detection flow:
1. Find Antigravity language server process
2. Extract CSRF token from process arguments (parseFlag)
3. Discover port from --port flag or via lsof/ss/netstat (tryPort)
4. Validate endpoint via verifyEndpoint - HTTP GET to Connect RPC health
5. Fetch quota data via FetchQuotas - single HTTP POST to GetUserStatus

Key types:
- Snapshot - captured quota data with provenance fields
- ModelQuota - per-model `*float64` remainingFraction (protobuf semantics: nil=missing, 0=exhausted), resetTime, label
- GroupedQuota - logical group (claude_gpt, gemini_pro, gemini_flash)

Data integrity:
- `remainingFraction` uses `*float64` to distinguish protobuf zero (0% = exhausted) from missing/null
- AI Credits (`availableCredits`, `promptCredits`, `flowCredits`) extracted from `GetUserStatus` API and stored in `ai_credits_json`
- LS payload uses `ideName: "antigravity"` (not `windsurf`) for correct data matching
- LS cache refreshes on ~60-120s timer; Quick Adjust lets users fine-tune stale values post-snap

## 2. internal/store/ - SQLite Ledger

Persists all application state in a local SQLite database.
Uses modernc.org/sqlite (pure Go, no CGo) for true single-binary cross-compilation.

### Schema Version History

| Version | Tables Added | Migration |
|---------|-------------|-----------|
| v1 | accounts, snapshots | Initial schema |
| v2 | subscriptions | Manual subscription tracking, 26 presets |
| v3 | config, activity_log, data_sources | Infrastructure: provenance, audit trail, sources |
| v4 | antigravity_reset_cycles | Per-model reset cycle tracking (Phase 7) |
| v5 | claude_snapshots | Claude Code rate limit data (Phase 9) |
| v6 | system_alerts | System-level alerts with hybrid TTL (Phase 10) |
| v7 | codex_snapshots, usage_sessions, usage_logs | Codex integration + sessions (Phase 11) |
| v8 | snapshots.ai_credits_json column | AI Credits tracking (Phase 12) |
| v9 | codex_snapshots.email column | Codex multi-account identity (Phase 12) |
| v10 | accounts: notes, tags, pinned_group | Account metadata (Phase 13) |
| v11 | accounts: credit_renewal_day | AI credit renewal tracking (Phase 13) |

### Tables (v11 - Current)

- accounts: id, email, plan_name, notes, tags, pinned_group, credit_renewal_day, created_at, updated_at
- snapshots: id, account_id, captured_at, email, plan_name, prompt_credits, monthly_credits, models_json, raw_json, capture_method, capture_source, source_id
- subscriptions: id, platform, category, plan_name, status, cost_monthly, currency, billing_cycle, email, next_renewal, trial_ends_at, notes, dashboard_url, status_page_url
- config: key, value, value_type, category, label, description, updated_at
- activity_log: id, timestamp, level, source, event_type, account_email, snapshot_id, details
- data_sources: id, name, source_type, enabled, config_json, last_capture, capture_count, created_at
- antigravity_reset_cycles: id, account_id, model_id, cycle_start, cycle_end, peak_usage, total_delta, snap_count
- claude_snapshots: id, captured_at, five_hour and seven_day rate limit fields
- system_alerts: id, severity, category, message, dismissed, created_at, expires_at
- codex_snapshots: id, captured_at, account_id, five_hour, seven_day, review quota fields
- usage_sessions: id, provider, started_at, ended_at, snap_count, duration_sec
- usage_logs: id, subscription_id, logged_at, amount, unit, notes

## 3. internal/readiness/ - Readiness Engine

Pure computation of readiness state from snapshot data. Zero I/O, zero network.

Input: Latest snapshot per account
Output: AccountReadiness with per-group status, staleness, reset countdowns

### Model Grouping Logic

Antigravity exposes per-model quotas. Niyantra groups them into 3 logical buckets:

| Group Key | Display Name | Model Match |
|-----------|-------------|-------------|
| claude_gpt | Claude + GPT | contains "claude" or "gpt" |
| gemini_pro | Gemini Pro | contains "gemini" but not "flash" |
| gemini_flash | Gemini Flash | contains "gemini" and "flash" |

For each group:
- Remaining % = average of remainingFraction across all models in group
- Reset time = earliest resetTime in the group
- Exhausted = any model in the group has remainingFraction <= 0

## 4. internal/web/ - Dashboard and API

Serves a 4-tab dashboard with embedded static assets and a REST API.

### Endpoints (30 REST)

| Method | Path | Description |
|--------|------|-------------|
| GET | / | Dashboard HTML |
| GET | /api/status | All accounts + readiness |
| POST | /api/snap | Trigger snapshot (tags source=ui) |
| GET | /api/history | Snapshot history (with provenance) |
| GET/POST | /api/subscriptions | List / create subscriptions |
| PUT/DELETE | /api/subscriptions/:id | Update / delete subscription |
| GET | /api/overview | Spend, renewals, insights |
| GET | /api/presets | 26 platform templates |
| GET | /api/export/csv | CSV download |
| GET/PUT | /api/config | Server config CRUD |
| GET | /api/activity | Activity log |
| GET | /api/mode | Current capture mode status |
| GET | /api/usage | Per-model intelligence + budget forecast |
| GET | /api/claude/status | Claude Code bridge status + rate limits |
| GET | /api/backup | Download database backup |
| POST | /api/notify/test | Send test OS notification |
| GET | /api/export/json | Full JSON export |
| GET | /api/alerts | Active system alerts |
| POST | /api/alerts/dismiss | Dismiss alert by ID |
| GET | /api/advisor | Switch advisor recommendation |
| GET | /api/codex/status | Codex detection + latest snapshot |
| POST | /api/codex/snap | Manual Codex snapshot |
| GET | /api/sessions | Usage sessions timeline |
| GET/POST | /api/usage-logs | Usage log CRUD |
| DELETE | /api/usage-logs/:id | Delete usage log |
| POST | /api/import/json | Import JSON with merge/dedup |
| GET | /api/accounts | List all tracked accounts |
| DELETE | /api/accounts/:id | Cascade delete account + all data |
| DELETE | /api/accounts/:id/snapshots | Clear snapshots only |
| DELETE | /api/snapshots/:id | Delete single snapshot |
| PATCH | /api/snap/adjust | Quick Adjust: fine-tune model quotas on a snapshot |

Stack: Go embed.FS + TypeScript (strict mode, 27 modules) bundled via esbuild into a single IIFE. Chart.js bundled locally from embedded assets.

## Data Flow

### Snap Flow (1 network call, full provenance)

```
User invokes "snap" (CLI or UI)
  |
  v
client.FetchQuotas(ctx)                <-- 1 HTTP POST to localhost (LS RPC)
  |
  v
Parse response: extract models (*float64 remainingFraction), email, plan, AI credits
  |
  v
Tag provenance:
  capture_method = "manual"
  capture_source = "cli" or "ui"
  source_id      = "antigravity"
  |
  v
store.GetOrCreateAccount(email)        <-- SQLite lookup/insert
  |
  v
store.InsertSnapshot(snap)             <-- SQLite insert (with provenance + ai_credits_json)
  |
  v
store.LogActivity("snap", ...)         <-- Activity log entry
  |
  v
Auto-link: create/update subscription  <-- If auto_link_subs=true
  |
  v
User may Quick Adjust via UI           <-- PATCH /api/snap/adjust (optional)
```

### Status Flow (0 network calls)

```
User invokes "status"
  |
  v
store.LatestSnapshotPerAccount()      <-- SQLite query
  |
  v
readiness.Calculate(snapshots)        <-- Pure computation
  |
  v
Print readiness table / return JSON
```

## Security Model

- No credentials stored: Niyantra does not store API keys, tokens, or passwords
- No outbound network: Only connects to 127.0.0.1 (local language server)
- Process detection: Reads process command lines via OS tools (ps/CIM/netstat)
- CSRF token: Extracted from process arguments, used for same-request auth to local LS
- TLS: Self-signed cert from language server, InsecureSkipVerify: true (localhost only)
- Dashboard auth: Optional HTTP basic auth via --auth user:pass flag
- Activity log: Full audit trail of every data mutation and config change

## Dependency Policy

| Dependency | Purpose | Why |
|-----------|---------|-----|
| modernc.org/sqlite | SQLite database | Pure Go, no CGo, cross-compile friendly |
| github.com/modelcontextprotocol/go-sdk | MCP server for AI agents | Official MCP Go SDK, stdio transport |
| Go stdlib | Everything else | HTTP server, JSON, templates, embed, crypto |

No other dependencies. No web frameworks, no ORM, no logging libraries.
Chart.js is bundled locally from embedded assets (no CDN dependency) for quota history visualization.
