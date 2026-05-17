# Security Model

> **Updated:** v0.27.0 · 7 providers + plugins · 4 notification channels · 5 security headers · Rate limiting

## What Niyantra Accesses

| Data | How | Why |
|------|-----|-----|
| Language server process list | `ps aux` / CIM / `Get-Process` | Detect running Antigravity instance |
| Language server command-line args | Process inspection | Extract CSRF token and port number |
| Local HTTP endpoint (`127.0.0.1`) | Single HTTP POST per snap | Fetch quota data via Connect RPC |
| Codex auth file (`~/.codex/auth.json`) | File read | OAuth token for Codex API polling |
| Claude settings (`~/.claude/settings.json`) | File read + optional patch | Statusline bridge for rate limits |
| Claude session logs (`~/.claude/projects/`) | File read | JSONL session parsing for token analytics |
| Cursor session token | File read from `~/.cursor-server/` | HTTP API authentication |
| Gemini CLI credentials | File read from `~/.config/gemini/` | OAuth for GCP API polling |
| GitHub Copilot PAT | User-provided in Settings UI | GitHub billing API authentication |
| Plugin scripts | Subprocess execution from `~/.niyantra/plugins/` | Execute external scripts in sandboxed subprocess |

## What Niyantra Does NOT Access

- No programmatic account switching (see "Why Not Account Switching" below)
- No user credentials stored or transmitted (except opt-in provider tokens stored locally)
- No telemetry, analytics, or phone-home
- No file system writes outside its own database directory

## Network Behavior

### Localhost Only (Core)
- **`niyantra snap`**: 1 HTTP POST to `https://127.0.0.1:{port}` (self-signed TLS, local LS)
- **`niyantra serve`**: Binds HTTP server on `localhost:9222` (configurable)
- **`niyantra mcp`**: stdio only, no network
- **`niyantra status`**: 0 network calls (reads from SQLite)

### External (Opt-In Provider Polling)
- **Codex**: HTTPS to `auth0.openai.com` (OAuth token refresh) + OpenAI API
- **Cursor**: HTTPS to `cursor.com/api/usage`
- **Gemini CLI**: HTTPS to GCP APIs (loadCodeAssist + retrieveUserQuota)
- **GitHub Copilot**: HTTPS to `api.github.com` (billing endpoints)

### External (Opt-In Notification Channels)
- **SMTP**: TCP to configured SMTP server (plain/STARTTLS/TLS)
- **Webhook**: HTTPS POST to configured endpoint (Discord/Telegram/Slack/ntfy)
- **WebPush**: HTTPS POST to push service (Chrome FCM, Firefox autopush, etc.)

> **Note:** All external network calls are opt-in and disabled by default. The dashboard works fully offline with only Antigravity LS detection.

## TLS

The Antigravity language server uses a self-signed certificate. Niyantra connects with `InsecureSkipVerify: true` because:
1. The connection is to `127.0.0.1` (localhost only)
2. The CSRF token provides request authentication
3. The alternative (no TLS) would be less secure

## Sensitive Configuration Masking

Config keys containing secrets are masked before API transmission. When returned via `GET /api/config` or `PUT /api/config` response, the actual values are replaced with `"configured"`.

### Static Keys

| Key | Purpose |
|-----|--------|
| `copilot_pat` | GitHub Personal Access Token |
| `cursor_session_token` | Cursor session authentication cookie |
| `gemini_client_secret` | Gemini CLI OAuth client secret |
| `smtp_pass` | SMTP authentication password |
| `smtp_user` | SMTP authentication username |
| `webhook_secret` | Webhook authentication secret (Telegram bot token, etc.) |
| `webpush_vapid_private` | VAPID P-256 private key |

### Dynamic Pattern Matching (Plugins)

Plugin config keys matching these suffix patterns are automatically masked:
`_api_key`, `_token`, `_secret`, `_password`, `_pat`, `_credential`

For example, `plugin_weather_api_key` is masked automatically.

## Browser Security Headers

All HTTP responses include the following security headers:

| Header | Value | Purpose |
|--------|-------|---------|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; ...` | Prevents XSS by restricting script sources |
| `X-Frame-Options` | `DENY` | Prevents clickjacking via framing |
| `X-Content-Type-Options` | `nosniff` | Prevents MIME type sniffing |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limits URL leakage in Referer header |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=(), payment=()` | Disables unnecessary browser features |

## Dashboard Authentication

Optional HTTP basic auth via `--auth user:pass` flag or `NIYANTRA_AUTH` environment variable. No session tokens, no cookies. The auth is per-request and not persisted.

**LAN Exposure Warning:** If `--bind 0.0.0.0` is used without `--auth`, Niyantra prints a visible warning to stderr advising the user to enable authentication before exposing the dashboard to the network.

## Rate Limiting

Per-IP in-memory token bucket rate limiter protects all mutation endpoints from abuse:

| Tier | Endpoints | Limit | Window |
|------|-----------|-------|--------|
| `snap` | `POST /api/snap`, `POST /api/snap/all` | 10 requests | 1 minute |
| `mutate` | `PUT /api/config`, `PATCH /api/snap/adjust` | 30 requests | 1 minute |
| `import` | `POST /api/import/json` | 2 requests | 1 minute |

When exceeded: `429 Too Many Requests` with `Retry-After` header. Zero external dependencies — uses `sync.Mutex` + background cleanup goroutine (stale buckets cleaned every 10 minutes).

## Config Type Validation

`PUT /api/config` validates values against their declared schema types before persistence:

| Type | Validation |
|------|-----------|
| `bool` | Only `"true"` or `"false"` accepted |
| `int` | Must parse as integer; range-checked per key (`poll_interval`: 30-3600, `retention_days`: 30-3650) |
| `float` | Must parse as float; range-checked per key (`notify_threshold`: 5-50) |
| `string` | No validation (passthrough) |

Rejects malformed input with `400 Bad Request` and a descriptive error message.

## Data Storage

- All data stored in a single SQLite file (default: `~/.niyantra/niyantra.db`)
- No encryption at rest (the database contains quota percentages, not credentials)
- Provider tokens stored in config table (masked in API, plaintext in SQLite)
- Backup/restore via `niyantra backup` / `niyantra restore`
- WebPush VAPID keys auto-generated on first subscribe (P-256 ECDSA)
- Plugin API keys stored in config table (masked in API, same treatment as other secrets)

## Cloud Sync Security (Planned — ADR-0002)

When cloud sync is enabled (opt-in):

| Concern | Mitigation |
|---------|------------|
| Data isolation | PocketBase Row-Level Security on all 12 synced collections |
| Auth | OAuth 2.0 PKCE (no client_secret on user's machine) |
| Token storage | OS-native keychain via `go-keyring` (Win/Mac/Linux) |
| Secrets in sync | **"Secrets Don't Sync" policy** — config keys with `syncable=0` never leave machine |
| Transport | TLS 1.3 via Caddy + Let's Encrypt (HTTPS everywhere) |
| MCP exposure | MCP reads local SQLite only — no direct cloud access from MCP |
| CORS | Eliminated via single-origin architecture |
| Admin access | PocketBase admin `/_/` restricted by IP whitelist in Caddy |
| GDPR | "Delete Cloud Data" button — cascades all 12 collections |
| At rest | Oracle Cloud boot volume encryption |

## Provenance

Every snapshot is tagged with:
- `capture_method`: `manual` or `auto`
- `capture_source`: `cli`, `ui`, or `server`
- `source_id`: which data source produced it
- `captured_at`: timestamp

This creates an audit trail — you can always verify *how* and *when* data entered the system.

## Reporting Vulnerabilities

If you discover a security issue, please email security@bhaskarjha.com rather than opening a public issue.

## Why NOT Account Switching

Niyantra deliberately avoids any feature that programmatically switches IDE accounts. This is a safety decision:

- **Google's Trust & Safety systems** use AI-driven anomaly detection that flags programmatic OAuth/RPC manipulation (e.g., `registerGdmUser`, automated token injection) as "botting" or "malicious behavior"
- **The punishment is account-wide**: a flagged violation can result in suspension of the user's **entire Google account** (Gmail, Drive, Calendar, YouTube — not just the IDE service)
- **Competitors that offer switching** (e.g., AG Switchboard's one-click account switch) expose users to this risk by injecting OAuth credentials into the Language Server process
- **Niyantra's principle: "Observe, never act"** — we read quota data, we never write to or manipulate external services

This applies equally to:
- Proactive token refresh (automated credential management)
- IDE account auto-sync (detecting and reacting to external switches)
- Any feature requiring `registerGdmUser` or similar RPC calls
