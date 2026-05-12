# Changelog

All notable changes to Niyantra are documented here.


## [0.16.0] - 2026-05-12

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

## [0.15.0] - 2026-05-09

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

## [0.14.0] - 2026-04-30

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

## [0.13.0] - 2026-04-20

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

## [0.12.0] - 2026-04-19

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

## [0.11.0] - 2026-04-18

### Added
- Codex/ChatGPT integration with OAuth polling and multi-quota tracking (5h, 7d, code review)
- Usage session detection with configurable idle timeout
- JSON import with additive merge and natural-key deduplication
- Manual usage logs per subscription
- `codex_status` MCP tool (total: 8 MCP tools)
- Schema v7: `codex_snapshots`, `usage_sessions`, `usage_logs` tables

## [0.10.0] - 2026-04-17

### Added
- Smart switch advisor with multi-factor scoring (remaining%, burn rate, reset time)
- Renewal calendar (CSS grid month view)
- JSON export for full data portability
- System alerts with hybrid TTL
- `analyze_spending` and `switch_recommendation` MCP tools
- Schema v6: `system_alerts` table

## [0.9.0] - 2026-04-16

### Added
- Claude Code statusline bridge (rate limit monitoring)
- OS-native notifications (cross-platform: PowerShell, osascript, notify-send)
- Database backup/restore (CLI + dashboard)
- Command palette (`Ctrl+K` with fuzzy search)
- Schema v5: `claude_snapshots` table

## [0.8.0] - 2026-04-15

### Added
- MCP server over stdio with 5 tools (quota, models, usage, budget, best_model)
- Official MCP Go SDK integration

## [0.7.0] - 2026-04-14

### Added
- Per-model reset cycle detection (3 methods: time-diff, fraction-jump, explicit reset)
- Usage rate forecasting and projected exhaustion
- Budget burn rate alerts
- Schema v4: `antigravity_reset_cycles` table

## [0.6.0] - 2026-04-13

### Added
- Auto-capture polling agent with ticker loop
- Configurable polling interval (30s-300s)
- Exponential backoff on failure
- Graceful shutdown

## [0.5.0] - 2026-04-12

### Added
- Settings tab with server-level config
- Search across subscriptions
- Keyboard shortcuts
- PWA manifest

## [0.4.0] - 2026-04-11

### Added
- Chart.js quota history visualization
- Budget threshold configuration
- Smart insights engine

## [0.3.0] - 2026-04-10

### Added
- Schema v3: `config`, `activity_log`, `data_sources` tables
- Snapshot provenance (capture_method, capture_source, source_id)
- Activity log with structured event tracking

## [0.2.0] - 2026-04-09

### Added
- Subscription CRUD with 26 platform presets
- Overview stats (total spend, category breakdown)
- CSV export
- Schema v2: `subscriptions` table

## [0.1.0] - 2026-04-08

### Added
- Initial release
- Antigravity language server detection (Windows CIM, macOS/Linux ps)
- Manual snapshot capture (`niyantra snap`)
- CLI status display (`niyantra status`)
- Web dashboard (`niyantra serve`)
- SQLite persistence with pure-Go driver
- Schema v1: `accounts`, `snapshots` tables
