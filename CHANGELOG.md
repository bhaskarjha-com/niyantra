# Changelog

All notable changes to Niyantra are documented here.

## [0.12.0] - 2026-04-19

### Added
- MIT License for open-source release
- `niyantra demo` command for sample data seeding
- Makefile with version injection (`make build`, `make run`, `make demo`)
- GitHub Actions CI (3 OS) and release workflows
- Unit tests for `readiness` and `advisor` packages (16 tests)
- SECURITY.md, CHANGELOG.md documentation
- Comparison table in README (vs onWatch, Wallos)
- "Who Is This For?" and use-case stories in documentation

### Changed
- README redesigned: hero + quickstart + user-story features + honest comparison
- ARCHITECTURE.md rewritten (fixed single-line encoding, updated to schema v7)
- VISION.md updated with market position, competitor landscape, use-case stories

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
