# Niyantra

**Track every AI subscription. Monitor every quota. Control every dollar.**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/macOS%20%7C%20Linux%20%7C%20Windows-grey?style=flat-square)]()
[![CI](https://img.shields.io/badge/CI-GitHub%20Actions-2088FF?style=flat-square&logo=github-actions&logoColor=white)](.github/workflows/ci.yml)

> Local-first. Zero daemon by default. Full provenance on every data point. All data stays on your machine.

<!-- ![Dashboard Screenshot](docs/screenshots/overview-dark.png) -->

---

## The Problem

Developers in 2026 spend **$200-600/month** across 8+ AI subscriptions. Each tool has its own dashboard, its own billing page, its own quota display. There is no unified view of what you're spending, which accounts have quota, or when renewals hit.

Quota trackers solve part of this, but they only watch API usage. What about the 5 other subscriptions you're paying for? The trial that auto-converted yesterday? The $30/month tool you forgot to cancel?

**Niyantra combines AI quota monitoring with subscription management, budget intelligence, and AI agent integration** in a single local-first binary.

## Who Is This For?

- **Multi-account developers** juggling work + personal AI accounts
- **AI power users** paying $100-500/month across multiple subscriptions
- **Privacy-conscious developers** who want local-only data, no cloud dashboards
- **Freelancers & contractors** tracking AI costs per client or project
- **Anyone tired of checking 5 different dashboards** to answer "am I ready to code?"

## Quick Start

```bash
# Build from source (Go 1.25+)
go build -o niyantra ./cmd/niyantra

# Capture your first Antigravity snapshot
./niyantra snap

# Launch the dashboard
./niyantra serve    # http://localhost:9222
```

Or with Make:
```bash
make build    # Build with version injection
make run      # Build + launch dashboard
make demo     # Seed sample data + launch (try before configuring)
```

---

## What Niyantra Does

### Know Your Quotas
Auto-capture Antigravity per-model quotas (Claude, Gemini, GPT) with rolling 5-hour reset detection. Track Codex/ChatGPT via OAuth API. Monitor Claude Code rate limits via statusline bridge. See who's ready, who's exhausted, and when resets happen.

### Control Your Spending
Track subscriptions across 26+ AI platforms with renewals, spending breakdowns, and CSV export. Set a monthly budget and get forecasts before you overspend. Visual renewal calendar so nothing surprises you.

### Let AI Help You Code Smarter
**Switch Advisor** ranks your accounts and tells you which one to use right now. **MCP Server** (8 tools) lets your AI agent check quotas, analyze spending, and get routing recommendations mid-task -- no context switching.

### Your Data, Your Machine
SQLite database. No cloud. No accounts. No tracking. No telemetry. Full provenance audit trail on every snapshot. MIT licensed.

---

## Dashboard

**4 tabs** -- Quotas, Subscriptions, Overview, Settings

| Tab | What it shows |
|-----|---------------|
| **Quotas** | Per-account readiness grid, per-model progress bars, reset countdowns, history chart |
| **Subscriptions** | Card view of all AI subs, search, 26 platform presets, CSV export |
| **Overview** | Monthly budget vs actual, switch advisor, Codex status, sessions timeline, renewal calendar |
| **Settings** | Auto-capture, polling interval, notifications, Claude bridge, Codex, backup/restore |

## How It Works

```
You're coding in Antigravity
    |
    v
Niyantra watches your quotas (manual snap or auto-polling)
    |
    v
Everything stored locally in SQLite (with full provenance)
    |
    v
Dashboard shows all accounts + subscriptions + budget
    |
    v
AI agents can query via MCP ("which account should I use?")
```

Each `snap` makes exactly **one HTTP call** to the local language server. No cloud APIs, no API keys, no rate-limit risk.

## CLI Reference

| Command | What it does | Network |
|---------|-------------|:---:|
| `niyantra snap` | Capture current Antigravity account's quota | 1 call |
| `niyantra status` | Show all accounts' readiness | 0 |
| `niyantra serve` | Launch web dashboard at `localhost:9222` | 0 |
| `niyantra mcp` | Start MCP server (stdio) for AI agents | 0 |
| `niyantra demo` | Seed database with sample data | 0 |
| `niyantra backup` | Create timestamped database backup | 0 |
| `niyantra restore <file>` | Restore from backup | 0 |
| `niyantra version` | Print version | 0 |

**Flags:** `--port 9222` `--db ~/.niyantra/niyantra.db` `--auth user:pass` `--debug`

## MCP Integration

Niyantra exposes quota intelligence to AI coding agents via the [Model Context Protocol](https://modelcontextprotocol.io).

**8 tools:** `quota_status` `model_availability` `usage_intelligence` `budget_forecast` `best_model` `analyze_spending` `switch_recommendation` `codex_status`

Add to Claude Desktop's config:
```json
{
  "mcpServers": {
    "niyantra": {
      "command": "path/to/niyantra",
      "args": ["mcp"]
    }
  }
}
```

Then ask: *"What's my Antigravity quota?"* or *"Which account should I use?"* or *"How much am I spending on AI this month?"*

## Comparison

| Feature | Niyantra | Quota trackers (e.g. onWatch) | Sub trackers (e.g. Wallos) |
|---------|----------|-------------------------------|---------------------------|
| AI quota monitoring | Antigravity + Codex + Claude | Up to 9 providers | -- |
| Subscription management | 26 AI platforms, renewals, CSV | -- | Generic subs |
| Budget forecasting | Monthly budget with projections | -- | Basic budget |
| Switch advisor (account routing) | Multi-factor scoring engine | -- | -- |
| MCP for AI agents | 8 tools over stdio | -- | -- |
| Renewal calendar | Visual month view | -- | -- |
| Activity audit trail | Full provenance on every data point | -- | -- |
| Zero-daemon default | Manual mode, opt-in auto | Daemon by default | N/A |
| License | MIT | GPL-3 | MIT |
| Single binary | Yes (Go, no CGo) | Yes | No (PHP/Docker) |

> **Note:** Quota tracker tools like onWatch excel at multi-provider coverage with support for 9+ providers. Niyantra focuses on combining quota data with subscription management, budget intelligence, and AI agent integration rather than maximizing provider count.

## Building from Source

**Prerequisites:** Go 1.25+

```bash
git clone https://github.com/bhaskarjha-com/niyantra.git
cd niyantra
make build
./niyantra serve
```

## Dependencies

| Dependency | Purpose |
|-----------|---------|
| [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) | Pure-Go SQLite -- no CGo, true single binary |
| [`go-sdk/mcp`](https://github.com/modelcontextprotocol/go-sdk) | Official MCP Go SDK for AI agent integration |
| Go stdlib | Everything else -- HTTP, JSON, embed, crypto |

No web frameworks. No ORMs. No npm. Chart.js loaded from CDN for visualization.

## Stats

| Metric | Value |
|--------|-------|
| Lines of code | ~14,000 (Go + JS + CSS + HTML) |
| Binary size | ~18 MB |
| External deps | 2 (sqlite + MCP SDK) |
| REST endpoints | 27 |
| MCP tools | 8 |
| Unit tests | 16 (readiness + advisor) |
| Schema version | v7 (11 tables) |

## Documentation

| Document | Content |
|----------|---------|
| [VISION.md](docs/VISION.md) | Product vision, market position, roadmap, use cases |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | System design, data flow, security model |
| [API_SPEC.md](docs/API_SPEC.md) | REST API reference (27 endpoints) |
| [DATA_MODEL.md](docs/DATA_MODEL.md) | SQLite schema v7, migrations |
| [SECURITY.md](docs/SECURITY.md) | What data is accessed, network behavior, threat model |
| [TESTING.md](docs/TESTING.md) | Test cases |
| [CONTRIBUTING.md](docs/CONTRIBUTING.md) | Development setup, code style, PR guidelines |
| [CHANGELOG.md](CHANGELOG.md) | Version history (v0.1.0 - v0.12.0) |

## License

[MIT](LICENSE) -- (c) 2026 Bhaskar Jha
