# Changelog

All notable changes to Niyantra are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions map to feature milestones, not semver.

## [0.29.0]

### Added
- **AI Cost Anomaly Detection (F5)** — Z-score engine (`internal/forecast/anomaly.go`) with rolling 30-day window. Detects spend spikes >2σ above average. Severity classification: warning (2-3σ), critical (>3σ). Budget projection estimates monthly excess. `GET /api/anomalies` endpoint. Frontend alert card with dismiss (localStorage). 8 new tests.
- **Notification Digest Mode (F8)** — `DigestBatcher` collects multiple quota alerts within a configurable time window (default 5 min) and flushes as a single summary. Batch-on-write pattern with early flush at 5 alerts. Thread-safe via `sync.Mutex` + `time.AfterFunc`. Delivers digests via all 4 channels (OS, SMTP, Webhook, WebPush). `ConfigureDigest()` engine method. 6 new tests.
- **Sparkline KPI Integration (F2)** — wired existing SVG sparkline renderer into Monthly AI Spend card (7-day trend) and Token Analytics KPI cards (Total Tokens, Est. Cost). Trend direction arrows (↑/↓/→) with semantic coloring.

### Changed
- Overview content order: Safe to Spend → Anomaly Card → Countdown → Advisor → Cost KPI → Token Analytics → ...
- CSS: sparkline integration classes (.kpi-with-sparkline, .token-kpi-spark), anomaly card styling (.anomaly-card, .anomaly-item)

## [0.28.0]

### Added
- **Safe to Spend guardrail (F1-UX)** — hero card showing remaining budget with semantic color coding (healthy/warning/over), daily burn rate, and projected overspend. CSP-safe event binding (no inline onclick). Full-width layout in overview grid.
- **SVG sparkline renderer (F2-UX)** — pure SVG micro-charts with trend direction detection (↑/↓/→). Zero dependencies, theme-aware via CSS variables.
- **Beautiful empty states (F3-UX)** — preview cards with sample data for Quotas, Subscriptions, and Overview tabs. Replaces blank screens for new users with actionable CTAs.
- **Usage streak hero card (F4-UX)** — prominent display of current streak, total snapshots, active days, and best streak. Rendered above the activity heatmap (replaces duplicate stats bar).
- **Reset countdown chips (F6-UX)** — inline countdown timers for providers with quota resets within 24h. Auto-refreshes every 60s.
- **Onboarding checklist (F7-UX)** — 5-step persistent checklist with auto-detection (snapshot, subscription, budget, notifications, overview tab). Progress bar, confetti on completion, localStorage persistence. Dismiss button.

### Fixed
- **Token Usage Analytics CSS** — added complete missing stylesheet for KPI cards, range selector, model distribution bars, daily burn chart, and breakdown chips. Previously had no CSS at all (pre-existing bug).
- **Gemini CLI model breakdown CSS** — added missing styles for per-model rows (`cursor-model-breakdown`, `cursor-model-row`, `quota-minibar`). Previously rendered as unstyled raw content. Applies to both Gemini CLI and Cursor providers.
- **CSP inline handler violations** — replaced all inline `onclick` handlers with CSP-safe `addEventListener` wiring for budget modal buttons.
- **Activity section data duplication** — removed old stats bar from heatmap (streak card now shows the same data in a better format).

### Changed
- Overview content order: Safe to Spend hero → Countdown → Advisor → Cost KPI → Token Analytics → Git Costs → Heatmap → Provider Health → Insights
- CSS entry point: 4 new stylesheets (ux-features, sparklines, onboarding, token-analytics)
- Provider model breakdown: upgraded from flexbox to CSS Grid layout with larger font (13px) and taller progress bars (7px)

## [0.27.0]

### Added
- **Rate limiting middleware** — per-IP token bucket rate limiter on all mutation endpoints. Three tiers: `snap` (10/min), `mutate` (30/min), `import` (2/min). Returns `429 Too Many Requests` with `Retry-After` header. Zero external dependencies — uses `sync.Mutex` + background cleanup goroutine.
- **Config type validation** — `PUT /api/config` validates values against declared schema types (`bool`, `int`, `float`) with range enforcement: `poll_interval` 30-3600, `retention_days` 30-3650, `notify_threshold` 5-50. Rejects malformed input with `400 Bad Request`.
- **Copilot Quota UI** — GitHub Copilot now renders on the Quotas tab with Premium and Chat dual usage bars, plan badge, status dot, and provider accent color (`#6e40c9`). Header badge counts Copilot. Provider/status filter includes Copilot.
- **Full import parity** — `POST /api/import/json` now imports all 7 provider snapshot types (Antigravity + Claude + Codex + Cursor + Gemini + Copilot + Plugin) with ±500ms dedup per provider. `ImportResult` extended with per-provider counters.
- **Parallel polling** — auto-capture agent now runs all 7 providers concurrently via `sync.WaitGroup` with semaphore(4) concurrency limiter. Poll cycle time reduced from O(sum) to O(max). Per-provider elapsed time logged at debug level.
- **Notification TTL reset** — `ResetGuard()` and `ResetAllGuards()` methods on the notification engine. Quick Adjust now calls `ResetAllGuards()` after successful adjustment, re-arming all quota alerts immediately.
- **Plugin test observability** — plugin test runs (`POST /api/plugins/{id}/run`) now log structured activity events via `LogInfo()` for both success and failure, including duration, provider, usage data, and persistence status.
- **28 new tests** — 16 import tests, 5 config validation tests, 5 rate limiter tests, 2 notification guard reset tests.

### Security
- **MCP endpoint hardened** — now enforces `--auth` basic auth and rejects cross-origin browser requests (Origin validation)
- **Activity log masking** — sensitive config values (PAT, passwords, VAPID keys) are masked with `***` in change logs
- **Config key validation** — `PUT /api/config` rejects unknown keys to prevent arbitrary data injection
- **Rate limiting** — all snap, config mutation, and import endpoints rate-limited per IP

### Fixed
- **JSON export** — now includes all 7 provider snapshot tables (Codex, Cursor, Gemini, Copilot, Plugin) + pretty-printed output
- **Notification guard** — uses 6h TTL expiry instead of permanent suppression for non-Antigravity providers
- **Quick Adjust** — uses direct `GetSnapshotByID` instead of O(N) scan; added `io.LimitReader` body limit; re-arms notification alerts
- **Config validation** — `notify_threshold` range check moved from `int` to `float` case to match schema declaration
- **Copilot status** — now included in unified `/api/status` response (parity with 4 other providers)
- **Stale TODOs** — removed completed TODO for rate limiting; converted 2 remaining TODOs to design notes (N23, N24); eliminated all TypeScript `any` types

### Changed
- 170+ tests across 18 files in 10 packages (+28 new tests from hardening sprint)
- Poll cycle architecture: sequential → concurrent with semaphore(4)
- Import architecture: Antigravity-only → all 7 providers with per-provider counters

## [0.26.0]

### Added
- **WebPush notifications (F19)** — browser push via VAPID (RFC 8292) + RFC 8291 encryption. Zero `x/crypto` dependency — HKDF implemented from stdlib `crypto/hmac` + `crypto/sha256`. Service Worker (`sw.js`) with subscribe/unsubscribe/test UI in Settings.
  - `webpush_subscriptions` table (schema v18)
  - `GET /api/webpush/vapid-key`, `POST /api/webpush/subscribe`, `DELETE /api/webpush/unsubscribe`, `GET /api/webpush/status`, `POST /api/webpush/test`
  - 14 tests in `webpush_test.go`
- **Webhook notifications (F22)** — multi-service webhook delivery with 4 adapters (Discord, Telegram, Slack, Generic/ntfy). Auto-format payloads per service. Severity-based color coding.
  - `webhook_enabled`, `webhook_service`, `webhook_url`, `webhook_secret` config keys (schema v17)
  - `POST /api/notify/test-webhook`
  - 12 tests in `webhook_test.go`
- **SMTP/Email notifications (F11)** — pure Go SMTP client supporting plain, STARTTLS, and TLS encryption. HTML-formatted quota alert emails.
  - 8 SMTP config keys (schema v16)
  - `POST /api/notify/test-smtp`
  - 8 tests in `smtp_test.go`
- **Notification engine refactor** — quad-channel async dispatch (OS + SMTP + Webhook + WebPush). Once-per-cycle guard. `engine.go` with threshold check and `OnNotify` callback. 8 tests in `engine_test.go`.
- **Plugin system (F18)** — `plugin_snapshots` table (schema v19). Plugin discovery, execution, and persistence.

### Changed
- Notification architecture upgraded from single-channel (OS-only) to quad-channel
- Config masking: `smtp_pass`, `webhook_secret`, `webpush_vapid_private` return `"configured"` in API (never expose secrets)
- Schema v16 → v19 (4 migrations)
- 148 total tests across 13 files in 10 packages

## [0.25.0]

### Added
- **GitHub Copilot provider (F15c)** — 7th tracked provider. PAT-based auth, GitHub billing API polling. Frontend settings with PAT input (masked in API).
  - `copilot_snapshots` table (schema v12)
  - `copilot_capture`, `copilot_pat` config keys (schema v15)
  - `GET /api/copilot/status`, `POST /api/copilot/snap`
- **Streamable HTTP MCP (F14)** — expose all 11 MCP tools over `POST /mcp` endpoint. SSE streaming, session management via `Mcp-Session-Id` header. Enables remote agent access.

## [0.24.0]

### Added
- **Git commit correlation (F16)** — AI cost per commit. Correlates git log timestamps with Claude Code JSONL sessions (±30 min window). Branch-level cost aggregation. `git_commit_costs` MCP tool.
  - `GET /api/git-costs`, `GET /api/git-costs/branches`
- **Token usage analytics (F13)** — multi-provider token intelligence. Claude JSONL session parser with per-turn input/output/cache token counting. Model-aware cost estimation using configured pricing. Daily aggregation.
  - `token_usage_daily` table (schema v13)
  - `GET /api/token-usage`, `POST /api/token-usage/parse`
  - `token_usage` MCP tool

### Fixed
- Fuzzy model ID matching for cost estimation (partial name match instead of exact)

## [0.23.0]

### Added
- **Docker deployment (F21)** — multi-stage Dockerfile (builder → distroless / Alpine). `docker-compose.yml` with volume persistence. Multi-arch support (linux/amd64, linux/arm64). `niyantra healthcheck` command for Docker health probes.
  - Makefile targets: `make docker`, `make docker-shell`, `make docker-run`

## [0.22.0]

### Added
- **Gemini CLI provider (F15b)** — OAuth credential discovery from `~/.config/gemini/`, 2-step API (loadCodeAssist + retrieveUserQuota). Full-stack: backend `internal/gemini/` + frontend settings + Quotas rendering.
  - `gemini_snapshots` table (schema v12)
  - `GET /api/gemini/status`, `POST /api/gemini/snap`

## [0.21.0]

### Added
- **Cursor provider (F15a)** — session token detection from filesystem, HTTP API polling to `cursor.com/api/usage`. Supports legacy request-based and new USD credit-based billing models.
  - `cursor_snapshots` table (schema v12)
  - `GET /api/cursor/status`, `POST /api/cursor/snap`
- **Schema v12** — unified account model. 3 new provider snapshot tables created in single migration. Agent polling refactored to split per-provider handlers.

### Fixed
- Cursor API client rewritten based on deep-dive analysis of actual API endpoints and auth flow

## [0.20.0]

### Added
- **Claude Code deep tracking (F15d)** — full JSONL session parser for per-turn token analytics (input/output/cache). Model-aware cost estimation. New `internal/claude/` package refactored from `claudebridge/`.
  - 7 tests in `deep_test.go`
- **Activity heatmap (F6)** — GitHub-style 365-day contribution grid. Color intensity reflects daily snapshot count. Configurable lookback via `heatmap_lookback_days` config key (schema v14).
  - `GET /api/heatmap`

## [0.19.0]

### Changed
- **CSS modularization** — monolithic `style.css` split into 22 domain CSS files bundled via esbuild. `make css`, `make css-prod`, `make css-watch` targets.

## [0.18.0]

### Changed
- **Frontend TypeScript migration** — 27 strict-mode TypeScript modules. IIFE-bundled via esbuild. Zero `@ts-nocheck` directives. `make js`, `make js-prod`, `make js-watch` targets.

## [0.17.0]

### Changed
- **Backend hardening** — monolithic 2,076-line `server.go` refactored into 11 focused files:
  - `server.go` (221 lines) — core lifecycle, constructor, route registration
  - `middleware.go` — basicAuth, securityMiddleware (CORS + Content-Type)
  - `helpers.go` — writeJSON, jsonError response helpers
  - `compute.go` — forecast/cost compute engines (no HTTP)
  - 6 domain handler files: `handlers_quota.go`, `handlers_config.go`, `handlers_ops.go`, `handlers_codex.go`, `handlers_data.go`, `handlers_forecast.go`
  - Existing `handlers_subscriptions.go` cleaned up
- **Go 1.22+ method routing** — all 49 route registrations use native `"METHOD /path"` syntax; 29 manual `r.Method` checks eliminated; automatic 405 Method Not Allowed
- **Go 1.22+ path parameters** — `r.PathValue("id")` replaces manual `strings.TrimPrefix` parsing on all dynamic routes

### Added
- **`GET /healthz`** — liveness endpoint returning version, schema version, account/snapshot counts
- **Environment variable configuration** — `NIYANTRA_PORT`, `NIYANTRA_BIND`, `NIYANTRA_DB`, `NIYANTRA_AUTH` (CLI flags take precedence)
- **`--bind` flag** — configurable bind address for Docker deployments (default: `127.0.0.1`)

## [0.16.0]

### Added
- **Account notes + tags** — per-account metadata: predefined tag palette (work, personal, primary, backup, shared, test, dev) + custom freeform tags, inline note editor (schema v10)
- **Pinned/favorite model** — star one group per account; pinned badge displays in collapsed view with percentage
- **Tag-based filtering** — dynamic tag filter strip in Quotas toolbar, click-to-filter with active state, auto-refresh on tag changes
- **Model pricing config** — editable per-model $/1M token pricing table in Settings, stored in server config, inline edit with Enter/Escape support
- **Notification wiring** — `notify/` engine connected to polling agent with once-per-cycle guard, OnReset callback for cycle-based clearing, proactive Codex monitoring, in-app system alerts
- **Quota time-to-exhaustion** — sliding-window linear regression forecasting with severity badges (Normal/Warning/Critical/Exhausted) per-group
- **Estimated cost tracking** — quota delta × model pricing = estimated session cost, displayed in Overview tab
- **Credit renewal day** — per-account renewal tracking with countdown badges and date picker (schema v11)
- **Live poll interval reload** — poll interval read inside ticker loop on every cycle, not just at startup

### Changed
- **Frontend modularized** — monolithic 4,265-line `app.js` decomposed into **27 strict-mode TypeScript modules** bundled via esbuild (IIFE format):
  - Entry point: `main.ts` (147 lines) with Window augmentation for inline handlers
  - 8 domain packages: `core/`, `quotas/`, `overview/`, `charts/`, `settings/`, `advanced/`, `types/`, root
  - 0 TypeScript compilation errors in strict mode, 0 `@ts-nocheck` directives
  - Bundle output: `app.js` (89 KB minified), build time: 7ms
  - Makefile targets: `make js`, `make js-prod`, `make js-watch`
- Schema version: v9 → v11 (accounts table gains `notes`, `tags`, `pinned_group`, `credit_renewal_day`)
- PATCH `/api/accounts/:id/meta` endpoint for account metadata updates (notes, tags, pinnedGroup, creditRenewalDay)

## [0.15.0]

### Added
- **Quick Adjust** — manual quota correction at group and model level:
  - Group-level ±5% buttons on Claude+GPT / Gemini Pro / Gemini Flash columns (appear on hover)
  - Model-level ±10%, ±5% buttons on individual model rows (appear on hover)
  - `PATCH /api/snap/adjust` endpoint for persisting adjustments
  - `UpdateSnapshotModels` store method with full DB round-trip
  - `latestSnapshotId` exposed in AccountReadiness for frontend reference
- Tests: `TestUpdateSnapshotModels`, `TestUpdateSnapshotModels_NotFound`

### Fixed
- **Removed Heartbeat RPC** — was a no-op (connection keep-alive, not a cache invalidator); removed to eliminate wasted latency before every snap
- **ideName fix** — LS payload corrected from `windsurf` to `antigravity`, ensuring proper quota data matching
- **Protobuf `*float64` handling** — correct treatment of `remainingFraction` where protobuf zero means 0% (exhausted), not null (missing data)
- **Reset-time-corrected aggregation** — group-level calculations (Claude+GPT total) now use time-adjusted model values instead of raw snapshot data
- **Account dimming logic** — dimmed by `isReady` flag instead of `allExhausted`, fixing visual inconsistency between fresh and stale snapshots
- **Provider collapse persistence** — collapse/expand state baked into HTML generation, preventing flash-expand on filter change
- **Subscription tab white flash** — pre-load data on init, removed re-fetch from tab switch handler
- **Tab animation flickering** — removed `tabFadeIn` CSS animation causing background color flash during DOM re-paints

### Removed
- `cloudapi.go` — dead-end direct Google Cloud API integration (unreliable token extraction from `state.vscdb`)
- `cmd/apitest/`, `cmd/dbprobe/`, `cmd/usstest/` — experimental test tools
- `Source` field from `UserStatusResponse`, `DataSource` field from `Snapshot` — no longer needed without cloud API dual-path

### Changed
- Advisor now detects "All Ready" state and shows "Stay" recommendation when health > 80%

## [0.14.0]

### Added
- **Provider-sectioned Quotas** — Antigravity, Codex, Claude Code shown in dedicated collapsible sections with provider-specific headers and color coding
- **Provider filter dropdown** — filter by provider (All / Antigravity / Codex / Claude)
- **Status filter** — filter accounts by readiness state (Ready / Low / Empty) with provider-aware logic for Codex and Claude
- **Split-button snap** — primary "Snap Now" (current account) + secondary dropdown "Snap All Sources" (all providers)
- **Hybrid subscription layout** — card + provider grouping with inline spend summary bar
- **Provider health cards** — Overview tab shows per-provider health status
- **Codex OIDC profile** — extract display name + profile picture from JWT `id_token` claims
- **Chart.js bundled locally** — removed CDN dependency, Chart.js served from embedded assets
- **Schema v9** — `email` column on `codex_snapshots` for multi-account Codex identity

### Changed
- Quotas tab completely redesigned from flat grid to provider-sectioned layout
- Subscriptions tab redesigned with progressive disclosure and provider grouping
- Overview tab now includes provider health cards and per-platform quick links
- UUID-style Codex display names truncated for readability
- Time-ago columns replace absolute timestamps in quota rows
- Dynamic advisor labels update based on current quota context

## [0.13.0]

### Added
- Real-time Google AI Credits monitoring embedded directly into the operation dashboard.
- Credit history stored historically (Schema v8 `ai_credits_json`) to support long-term burn rate analysis.
- Twin-axis support in the usage intelligence visualization chart bringing usage percentages and absolute API credits to the same timeline.
- A new interactive search bar and status filter directly above the quota table layout.
- Standalone CLI utility (`scripts/dump_antigravity_payload.go`) for extracting deep backend API unmapped JSON payloads natively from the database.

### Changed
- Quota table upgraded to a dynamic 6-column sortable grid (Account | Claude + GPT | Gemini Pro | Gemini Flash | AI Credits | Status).
- Replaced the static legacy "✦ 500" prompt credits indicator with live, color-coded AI Credit metrics pulled directly from the `GetUserStatus` API (`userTier.availableCredits`).
- Fully restructured storage `Snapshot` pipeline capturing the `OriginalRawJSON` buffer dynamically inline, preventing internal schema mapping dropouts.

## [0.12.0]

### Added
- MIT License for open-source release
- `niyantra demo` command for sample data seeding
- Makefile with version injection (`make build`, `make run`, `make demo`)
- GoReleaser config for automated cross-platform releases (6 binaries)
- `install.sh` one-liner installer for macOS/Linux
- GitHub Actions CI (3 OS) and GoReleaser-based release workflow
- Unit tests for `readiness` and `advisor` packages (16 tests)
- USER_GUIDE.md — complete end-user feature guide (516 lines)
- SECURITY.md — data access model and threat documentation
- CHANGELOG.md — version history from v0.1.0
- FAQ / Troubleshooting section in README
- GitHub issue templates (bug report, feature request)
- `go install` support for direct installation
- Honest comparison table in README (vs onWatch, Wallos)
- "Who Is This For?" and use-case stories in documentation

### Changed
- README redesigned: 4 install methods, "Try It Now", FAQ, feature groups
- ARCHITECTURE.md rewritten (fixed single-line encoding, updated to schema v7)
- VISION.md updated with market position, competitor landscape, use-case stories
- CONTRIBUTING.md rewritten with full project layout (12 packages), Make targets, testing section

## [0.11.0]

### Added
- Codex/ChatGPT integration with OAuth polling and multi-quota tracking (5h, 7d, code review)
- Usage session detection with configurable idle timeout
- JSON import with additive merge and natural-key deduplication
- Manual usage logs per subscription
- `codex_status` MCP tool (total: 8 MCP tools)
- Schema v7: `codex_snapshots`, `usage_sessions`, `usage_logs` tables

## [0.10.0]

### Added
- Smart switch advisor with multi-factor scoring (remaining%, burn rate, reset time)
- Renewal calendar (CSS grid month view)
- JSON export for full data portability
- System alerts with hybrid TTL
- `analyze_spending` and `switch_recommendation` MCP tools
- Schema v6: `system_alerts` table

## [0.9.0]

### Added
- Claude Code statusline bridge (rate limit monitoring)
- OS-native notifications (cross-platform: PowerShell, osascript, notify-send)
- Database backup/restore (CLI + dashboard)
- Command palette (`Ctrl+K` with fuzzy search)
- Schema v5: `claude_snapshots` table

## [0.8.0]

### Added
- MCP server over stdio with 5 tools (quota, models, usage, budget, best_model)
- Official MCP Go SDK integration

## [0.7.0]

### Added
- Per-model reset cycle detection (3 methods: time-diff, fraction-jump, explicit reset)
- Usage rate forecasting and projected exhaustion
- Budget burn rate alerts
- Schema v4: `antigravity_reset_cycles` table

## [0.6.0]

### Added
- Auto-capture polling agent with ticker loop
- Configurable polling interval (30s-300s)
- Exponential backoff on failure
- Graceful shutdown

## [0.5.0]

### Added
- Settings tab with server-level config
- Search across subscriptions
- Keyboard shortcuts
- PWA manifest

## [0.4.0]

### Added
- Chart.js quota history visualization
- Budget threshold configuration
- Smart insights engine

## [0.3.0]

### Added
- Schema v3: `config`, `activity_log`, `data_sources` tables
- Snapshot provenance (capture_method, capture_source, source_id)
- Activity log with structured event tracking

## [0.2.0]

### Added
- Subscription CRUD with 26 platform presets
- Overview stats (total spend, category breakdown)
- CSV export
- Schema v2: `subscriptions` table

## [0.1.0]

### Added
- Initial release
- Antigravity language server detection (Windows CIM, macOS/Linux ps)
- Manual snapshot capture (`niyantra snap`)
- CLI status display (`niyantra status`)
- Web dashboard (`niyantra serve`)
- SQLite persistence with pure-Go driver
- Schema v1: `accounts`, `snapshots` tables
