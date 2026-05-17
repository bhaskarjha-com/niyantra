# ADR-0002: Cloud Sync Architecture

**Status:** Accepted  
**Date:** 2026-05-17  
**Authors:** Bhaskar Jha  
**Supersedes:** `draft/cloud/00-10` (written 2026-05-01 at v0.15.0, Schema v9)

## Context

Niyantra is a local-first AI operations dashboard that currently runs as a single Go binary
with an embedded SQLite database. Users have requested multi-machine synchronization to
access their quota data, subscription tracking, and analytics from multiple workstations.

The original cloud architecture was designed when the project had 4 tables, 1 provider,
and a monolithic JavaScript frontend. Since then, the project has grown to 19 tables,
7 providers, 30 TypeScript modules, 12 MCP tools, 4 notification channels, and 40+ config
keys (many containing secrets like PATs, SMTP passwords, and VAPID keys).

### Key Requirements

1. **Local-first stays:** Cloud sync is additive. The app MUST work fully offline.
2. **Single-binary philosophy:** `go build ./cmd/niyantra` must still produce one executable.
3. **Zero daemon by default:** No background services unless user opts in.
4. **Secret safety:** Provider credentials (PATs, tokens, passwords) must never leave the machine.
5. **Minimal dependencies:** Currently only `modernc.org/sqlite` + MCP Go SDK.

## Decision

### Cloud Backend: PocketBase

PocketBase is selected as the cloud backend. It is a single Go binary with embedded SQLite,
built-in OAuth (15+ providers), Server-Sent Events for realtime subscriptions, row-level
security rules, and an admin dashboard.

**Alternatives evaluated and rejected:**

| Alternative | Rejection Reason |
|-------------|-----------------|
| Supabase | PostgreSQL (not SQLite), hosted-only, $25/mo minimum |
| Turso/libSQL | Provides sync but not auth, admin UI, or realtime subscriptions |
| Custom Go API | 3-6 months to build auth + realtime + storage |
| Convex | No Go SDK, cloud-first (not local-first) |
| PowerSync | Requires PostgreSQL backend |
| ElectricSQL | Requires PostgreSQL backend |

### Sync Protocol: Selective Push + SSE Subscribe

Of 19 local tables, 12 are classified as "sync-eligible" and 7 as "local-only":

- **Sync:** accounts, snapshots (all 7 providers), subscriptions, config (non-secret keys only),
  activity_log, token_usage, plugin_snapshots
- **Local-only:** data_sources, system_alerts, usage_sessions, usage_logs,
  antigravity_reset_cycles, webpush_subscriptions, sync_queue

Conflict resolution uses **Last-Write-Wins** for mutable data (accounts, subscriptions, config)
and **append-only with UUID dedup** for time-series data (all snapshot tables, activity_log).

### Secret Management: "Secrets Don't Sync" Policy

Config keys are classified into syncable and local-only via a new `syncable` column (schema v20).
Any key that is masked in the API response (copilot_pat, smtp_pass, webhook_secret,
webpush_vapid_private, plugin API keys) is marked `syncable=0` and never leaves the machine.

### Authentication: Google + GitHub OAuth via PKCE

PocketBase handles OAuth complexity. The desktop app implements Authorization Code Flow
with PKCE (public client, no client_secret). Tokens stored in OS-native keychain via
`go-keyring`. GitHub OAuth included because we already use GitHub PAT for Copilot integration.

### Desktop Wrapper: Deferred

Wails v3 remains the recommended framework but is deferred to a future sprint. Cloud sync
works entirely in browser mode. Shipping on an alpha framework is unnecessary risk.

### Hosting: Oracle Cloud Always Free (Mumbai)

Oracle's Always Free tier (4 ARM OCPU, 24GB RAM, 200GB storage) is selected as primary host,
region `ap-mumbai-1` (lowest latency for primary user, good capacity availability).
Risks (idle reclamation, account termination) are mitigated by PAYG upgrade, external
keepalive, automated backups, and a documented migration runbook. Hetzner ($4.50/mo)
is the documented backup option.

### Domain: Single-Origin Architecture

All services hosted under `niyantra.bhaskarjha.dev` (single first-level subdomain).
PocketBase serves the API, landing page, and cloud dashboard from `pb_public/`. This avoids:
- Second-level subdomain SSL issues (Cloudflare free cert doesn't cover `api.niyantra.*`)
- CORS configuration (same origin = no cross-origin requests)
- Extra DNS records (one A record to Oracle Cloud IP)

URL structure:
- `/` — Landing page (marketing, public)
- `/app/` — Cloud dashboard SPA (auth required, full CRUD + analytics)
- `/api/*` — PocketBase REST API (auth required)
- `/privacy`, `/terms` — Legal pages (public, required for OAuth)
- `/_/` — PocketBase Admin (admin-only, IP-restricted via Caddy)

### Cloud Dashboard: Full Management Console (~85% of Local)

The cloud dashboard is NOT read-only. It provides:
- **Full CRUD**: subscriptions, account notes/tags, budget, model pricing, Quick Adjust
- **Full analytics**: quota cards, heatmap, token usage, cost charts, forecast, advisor
- **Cloud-exclusive**: cross-machine analytics, fleet overview, scheduled emails, infinite history, data continuity
- **Local-only** (cannot work remotely): snap capture, plugin execution, MCP, git costs, secret config

PocketBase hooks enable cloud-native features:
- `onRecordAfterCreate` on snapshot collections → fire quota alerts even when source machine is idle
- Cron job → weekly AI spend summary email
- No retention cleanup → infinite historical data (vs. 90-day local default)

### Schema: v20 Migration

Adds UUID, machine_id, and synced_at columns to all 12 synced tables. Adds `syncable` column
to config table. Creates `sync_queue` table for offline resilience. UUID backfill for existing
rows uses stdlib `crypto/rand` (no new dependency).

## Consequences

### Positive

- Multi-machine sync enables the #1 most-requested feature
- Cloud dashboard is a full ~85% management console, not a read-only viewer
- 5 cloud-exclusive features (cross-machine analytics, fleet view, scheduled emails, infinite history, data continuity) add genuine Pro tier value
- PWA mobile access with zero extra framework (existing sw.js extends)
- Cloud sync is the revenue gate (Free = local, Pro = cloud) — enables sustainability
- Only 1 new dependency (go-keyring) — consistent with minimal-dep philosophy
- Data isolation guaranteed by PocketBase row-level security
- PocketBase hooks enable server-side intelligence (alerts, cron reports) that run 24/7

### Negative

- PocketBase is pre-v1.0, single-maintainer (mitigated: data is standard SQLite, portable)
- Oracle Free tier has termination risk (mitigated: backups + migration plan)
- go-keyring may not work in all Linux environments (mitigated: env var fallback)
- 12 PocketBase collections to maintain (mitigated: auto-created via admin UI)

### Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| PocketBase breaking change on upgrade | Medium | Pin version, test in staging |
| Oracle account termination | High | PAYG + offsite backups + Hetzner fallback |
| Secret leakage via sync | Critical | `syncable=0` on all secret keys, enforced at engine level |
| Service worker conflict (push + cache) | Low | Single sw.js with both event handlers |

## References

- Original cloud docs: `draft/cloud/00_overview.md` through `10_monetization.md`
- Research artifact: `cloud_sync_research.md` (full 10-component analysis)
- PocketBase: https://pocketbase.io
- go-keyring: https://github.com/zalando/go-keyring
- Oracle Always Free: https://www.oracle.com/cloud/free/
