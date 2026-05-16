# Architecture: Niyantra

> **Updated:** v0.26.1 · Schema v19 · 19 tables · 60+ REST endpoints · 132 tests

## System Overview

```
User Interfaces
  CLI (snap/status/serve/mcp/demo/backup/restore)
  Dashboard (4 tabs, embedded web app, PWA)
  MCP Server (12 tools, stdio + Streamable HTTP)
          |
Application Layer
  agent/        - polling loop + session management
  client/       - LS detection + quota fetch (Connect RPC)
  codex/        - OAuth + Codex API polling + OIDC JWT parsing
  claude/       - deep session parser + statusline bridge + settings patch
  cursor/       - session token auth + HTTP API polling
  gemini/       - OAuth + GCP billing/quota APIs
  copilot/      - GitHub PAT + Copilot billing endpoints
  advisor/      - switch recommendation engine
  tracker/      - cycle detection + intelligence + sessions
  readiness/    - pure readiness computation (reset-time-corrected)
  notify/       - quad-channel notifications (OS + SMTP + Webhook + WebPush)
  forecast/     - cost + TTX forecasting
  costtrack/    - blended model pricing + cost calculation
  tokenusage/   - Claude Code JSONL token analytics
  gitcorr/      - git commit ↔ token usage cost correlation
          |
Storage Layer
  store/  - SQLite v19 (19 tables, 24 Go files)
  config  - typed key-value settings (74+ config keys)
  Pure Go: modernc.org/sqlite (no CGo)
```

## Package Dependency Graph

```
cmd/niyantra/main.go
  +-- client       (detect Antigravity LS, fetch quotas via Connect RPC)
  +-- store        (SQLite, all persistence — 19 tables, 24 files)
  +-- web          (HTTP server + dashboard — 19 Go files, 30 TS modules)
  |    +-- agent        (polling loop, backoff, graceful shutdown)
  |    +-- tracker      (cycles + sessions)
  |    +-- advisor      (switch engine)
  |    +-- codex        (ChatGPT integration)
  |    +-- claude       (deep JSONL parser + statusline bridge)
  |    +-- cursor       (Cursor Pro quota polling)
  |    +-- gemini       (Gemini CLI OAuth + GCP APIs)
  |    +-- copilot      (GitHub Copilot billing)
  |    +-- notify       (OS + SMTP + Webhook + WebPush — 4 channels)
  |    +-- costtrack    (blended pricing engine)
  |    +-- forecast     (TTX + cost forecasting)
  |    +-- tokenusage   (Claude JSONL token analytics)
  |    +-- gitcorr      (git commit cost correlation)
  |    +-- plugin       (plugin discovery, manifest validation, subprocess exec)
  +-- mcpserver    (12 MCP tools over stdio + Streamable HTTP)
  |    +-- store, tracker, advisor, codex, readiness, forecast
  +-- readiness    (pure computation, zero I/O)
```

## 1. internal/client/ — Antigravity Language Server Client

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
4. Validate endpoint via verifyEndpoint — HTTP GET to Connect RPC health
5. Fetch quota data via FetchQuotas — single HTTP POST to GetUserStatus

Key types:
- Snapshot — captured quota data with provenance fields
- ModelQuota — per-model `*float64` remainingFraction (protobuf semantics: nil=missing, 0=exhausted), resetTime, label
- GroupedQuota — logical group (claude_gpt, gemini_pro, gemini_flash)

Data integrity:
- `remainingFraction` uses `*float64` to distinguish protobuf zero (0% = exhausted) from missing/null
- AI Credits (`availableCredits`, `promptCredits`, `flowCredits`) extracted from `GetUserStatus` API and stored in `ai_credits_json`
- LS payload uses `ideName: "antigravity"` (not `windsurf`) for correct data matching
- LS cache refreshes on ~60-120s timer; Quick Adjust lets users fine-tune stale values post-snap

## 2. internal/store/ — SQLite Ledger

Persists all application state in a local SQLite database.
Uses modernc.org/sqlite (pure Go, no CGo) for true single-binary cross-compilation.

### Schema Version History

| Version | Tables / Changes | Migration |
|---------|-----------------|-----------|
| v1 | `accounts`, `snapshots` | Initial schema |
| v2 | `subscriptions` | Manual subscription tracking, 26 presets |
| v3 | `config`, `activity_log`, `data_sources` | Infrastructure: provenance, audit trail, sources |
| v4 | `antigravity_reset_cycles` | Per-model reset cycle tracking (Phase 7) |
| v5 | `claude_snapshots` | Claude Code rate limit data (Phase 9) |
| v6 | `system_alerts` | System-level alerts with hybrid TTL (Phase 10) |
| v7 | `codex_snapshots`, `usage_sessions`, `usage_logs` | Codex integration + sessions (Phase 11) |
| v8 | `snapshots.ai_credits_json` column | AI Credits tracking (Phase 12) |
| v9 | `codex_snapshots.email` column | Codex multi-account identity (Phase 12) |
| v10 | `accounts.notes`, `tags`, `pinned_group` | Account metadata (Phase 13) |
| v11 | `accounts.credit_renewal_day` | AI credit renewal tracking (Phase 13) |
| v12 | `cursor_snapshots`, `gemini_snapshots`, `copilot_snapshots` | 3 new providers (Phase 14) |
| v13 | `token_usage_daily` | Claude deep token analytics (Phase 14) |
| v14 | `config`: `heatmap_lookback_days` | Activity heatmap config (Phase 14) |
| v15 | `config`: `copilot_pat`, `copilot_capture` | GitHub Copilot integration (Phase 15) |
| v16 | `config`: 8 SMTP keys | SMTP/Email notifications (Phase 16, F11) |
| v17 | `config`: 4 webhook keys | Webhook notifications (Phase 16, F22) |
| v18 | `webpush_subscriptions`, 3 config keys | WebPush notifications (Phase 16, F19) |
| v19 | `plugin_snapshots` table, plugin config keys | Plugin system (Phase 16, F18) |

### Tables (19 — Current)

- `accounts` — identity (email, plan, notes, tags, pinned_group, credit_renewal_day)
- `snapshots` — quota captures with provenance (account, models, ai_credits)
- `subscriptions` — manual subscription tracking (26 presets)
- `config` — 74 typed key-value settings (bool/int/float/string/json)
- `activity_log` — structured audit trail
- `data_sources` — source registry (antigravity, claude_code, codex, cursor, gemini, copilot)
- `antigravity_reset_cycles` — per-model cycle intelligence
- `claude_snapshots` — rate limit snapshots (5h/7d)
- `system_alerts` — dismissible alerts with hybrid TTL
- `codex_snapshots` — ChatGPT quota snapshots (5h/7d/review)
- `usage_sessions` — detected sessions per provider
- `usage_logs` — manual usage tracking
- `cursor_snapshots` — Cursor quota snapshots (requests/USD credits)
- `gemini_snapshots` — Gemini CLI quota snapshots
- `copilot_snapshots` — GitHub Copilot usage snapshots
- `token_usage_daily` — Claude Code per-day token analytics
- `git_commit_costs` — (virtual, via query) git ↔ session correlation
- `webpush_subscriptions` — browser push subscription storage
- `plugin_snapshots` — plugin-captured data snapshots

## 3. internal/readiness/ — Readiness Engine

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

## 4. internal/notify/ — Quad-Channel Notification Engine

Provides alert delivery for quota warnings via 4 independent channels:

| Channel | Implementation | Transport |
|---------|----------------|-----------|
| OS-native | `notify.go` | PowerShell (Win), osascript (macOS), notify-send (Linux) |
| SMTP email | `smtp.go` | Pure Go `net/smtp` + `crypto/tls` (plain/STARTTLS/TLS) |
| Webhook | `webhook.go` | Pure Go `net/http` — Discord, Telegram, Slack, Generic |
| WebPush | `webpush.go` | Pure Go VAPID (RFC 8292) + RFC 8291 encryption |

**Engine** (`engine.go`): threshold-gated, once-per-cycle guard, all async channels fire in goroutines.

**Zero dependencies**: WebPush implements HKDF (RFC 5869) from stdlib `crypto/hmac` + `crypto/sha256`. No `x/crypto` import.

## 5. internal/web/ — Dashboard and API

Serves a 4-tab dashboard with embedded static assets and a REST API.

### File Structure (19 Go files)

| File | Responsibility |
|------|----------------|
| `server.go` | Server struct, lifecycle, route table |
| `middleware.go` | basicAuth, securityMiddleware (CORS + Content-Type) |
| `helpers.go` | writeJSON, jsonError response helpers |
| `compute.go` | Forecast/cost compute engines (no HTTP) |
| `handlers_quota.go` | status, snap, history, usage |
| `handlers_config.go` | config CRUD, activity, mode, onConfigChanged |
| `handlers_ops.go` | healthz, Claude, backup, notify, export/import, alerts, advisor, webpush |
| `handlers_codex.go` | Codex status/snap, sessions, usage logs |
| `handlers_data.go` | accounts, snapshots, snap adjust, model pricing |
| `handlers_forecast.go` | cost and TTX forecast endpoints |
| `handlers_subscriptions.go` | subscription CRUD, overview, presets, CSV |
| `handlers_cursor.go` | Cursor status/snap endpoints |
| `handlers_gemini.go` | Gemini status/snap endpoints |
| `handlers_copilot.go` | Copilot status/snap endpoints |
| `handlers_plugins.go` | Plugin discovery, execution, config |
| `handlers_heatmap.go` | Activity heatmap data (365 days) |
| `embed_prod.go` / `embed_dev.go` | Build-tag-switched static FS |

### Endpoints (60 REST)

60 REST API endpoints organized by domain. Full documentation in `docs/API_SPEC.md`.

Stack: Go embed.FS + TypeScript (strict mode, 30 modules) bundled via esbuild into a single IIFE. Chart.js bundled locally from embedded assets.

## 6. Providers (7)

| Provider | Package | Auth | Method |
|----------|---------|------|--------|
| Antigravity | `client/` | CSRF token from process args | Connect RPC to local LS |
| Codex/ChatGPT | `codex/` | OAuth from `~/.codex/auth.json` | HTTPS to OpenAI API |
| Claude Code | `claude/` | None (local files) | JSONL session parsing + statusline bridge |
| Cursor | `cursor/` | Session token from `~/.cursor-server/` | HTTPS to cursor.com API |
| Gemini CLI | `gemini/` | OAuth from `~/.config/gemini/` | HTTPS to GCP APIs |
| GitHub Copilot | `copilot/` | GitHub PAT | HTTPS to GitHub billing API |
| Manual | `store/` | N/A | User input via subscription form |
| Plugins | `plugin/` | Plugin-specific (API keys in config) | Subprocess exec with JSON protocol |

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

### Notification Flow (quad-channel, async)

```
Engine.CheckQuota(model, remaining%) triggered by polling loop
  |
  v
Guard check: already notified this cycle? → skip
  |
  v
Channel 1: OS notification (sync)     → PowerShell/osascript/notify-send
Channel 2: SMTP email (goroutine)     → net/smtp + TLS
Channel 3: Webhook (goroutine)        → net/http POST to Discord/Telegram/Slack/ntfy
Channel 4: WebPush (goroutine)        → VAPID + RFC 8291 encrypted push
  |
  v
Mark model as notified for this cycle
  |
  v
Create system_alert + activity_log entry
```

## Security Model

- No credentials stored: Niyantra does not store API keys or passwords (except opt-in provider tokens)
- No outbound network: Core features connect only to 127.0.0.1 (local language server)
- Provider polling (opt-in): HTTPS to provider APIs using locally-stored tokens
- Sensitive config masking: `copilot_pat`, `smtp_pass`, `webhook_secret`, `webpush_vapid_private` returned as `"configured"` in GET
- CSRF token: Extracted from process arguments, used for same-request auth to local LS
- TLS: Self-signed cert from language server, InsecureSkipVerify: true (localhost only)
- Dashboard auth: Optional HTTP basic auth via --auth user:pass flag
- Environment variables: NIYANTRA_PORT, NIYANTRA_BIND, NIYANTRA_DB, NIYANTRA_AUTH (CLI flags take precedence)
- Activity log: Full audit trail of every data mutation and config change

## Dependency Policy

| Dependency | Purpose | Why |
|-----------|---------|-----|
| modernc.org/sqlite | SQLite database | Pure Go, no CGo, cross-compile friendly |
| github.com/modelcontextprotocol/go-sdk | MCP server for AI agents | Official MCP Go SDK, stdio + HTTP transport |
| Go stdlib | Everything else | HTTP server, JSON, templates, embed, crypto, TLS, SMTP |

No other dependencies. No web frameworks, no ORM, no logging libraries.
Chart.js is bundled locally from embedded assets (no CDN dependency) for quota history visualization.
WebPush uses stdlib crypto exclusively (ECDSA, AES-GCM, HMAC-SHA256) — zero x/crypto.
