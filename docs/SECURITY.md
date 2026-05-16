# Security Model

> **Updated:** v0.26.0 · 7 providers · 4 notification channels

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

The following config keys contain secrets. When returned via `GET /api/config`, the actual values are replaced with `"configured"` to prevent exposure:

| Key | Purpose |
|-----|--------|
| `copilot_pat` | GitHub Personal Access Token |
| `smtp_pass` | SMTP authentication password |
| `webhook_secret` | Webhook authentication secret (Telegram bot token, etc.) |
| `webpush_vapid_private` | VAPID P-256 private key |

## Dashboard Authentication

Optional HTTP basic auth via `--auth user:pass` flag or `NIYANTRA_AUTH` environment variable. No session tokens, no cookies. The auth is per-request and not persisted.

## Data Storage

- All data stored in a single SQLite file (default: `~/.niyantra/niyantra.db`)
- No encryption at rest (the database contains quota percentages, not credentials)
- Provider tokens stored in config table (masked in API, plaintext in SQLite)
- Backup/restore via `niyantra backup` / `niyantra restore`
- WebPush VAPID keys auto-generated on first subscribe (P-256 ECDSA)

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
