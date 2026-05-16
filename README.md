# Niyantra

**Track every AI subscription. Monitor every quota. Control every dollar.**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?style=flat-square)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/bhaskarjha-com/niyantra/ci.yml?style=flat-square&label=CI)](https://github.com/bhaskarjha-com/niyantra/actions)
[![Release](https://img.shields.io/github/v/release/bhaskarjha-com/niyantra?style=flat-square)](https://github.com/bhaskarjha-com/niyantra/releases)
[![Platform](https://img.shields.io/badge/macOS%20%7C%20Linux%20%7C%20Windows-grey?style=flat-square)](#install)

> Local-first. Zero daemon by default. Full provenance on every data point. MIT licensed.



---

## What Is Niyantra?

Developers in 2026 spend **$200-600/month** across 8+ AI tools. Each has its own quota dashboard, its own billing page, its own renewal date. Quota trackers show you API usage. Finance apps show you bank transactions. **Nothing shows you both.**

**Niyantra** is a local-first dashboard that **combines AI quota monitoring with subscription management, budget intelligence, and AI agent integration** in a single binary. No cloud. No accounts. All data on your machine.

## Install

### Quick Install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/bhaskarjha-com/niyantra/main/install.sh | sh
```

### Quick Install (Windows PowerShell)

```powershell
# Download and extract latest release
$release = (Invoke-RestMethod "https://api.github.com/repos/bhaskarjha-com/niyantra/releases/latest").tag_name -replace '^v',''
$url = "https://github.com/bhaskarjha-com/niyantra/releases/latest/download/niyantra_${release}_windows_amd64.zip"
Invoke-WebRequest -Uri $url -OutFile "$env:TEMP\niyantra.zip"
Expand-Archive -Path "$env:TEMP\niyantra.zip" -DestinationPath "$env:LOCALAPPDATA\niyantra" -Force
# Add to PATH (run once)
$p = [Environment]::GetEnvironmentVariable("Path","User"); if ($p -notlike "*niyantra*") { [Environment]::SetEnvironmentVariable("Path","$p;$env:LOCALAPPDATA\niyantra","User") }
```

### Go Install (all platforms)

```bash
go install github.com/bhaskarjha-com/niyantra/cmd/niyantra@latest
```

### Download Binary

Pre-built binaries for macOS, Linux, and Windows are available on the [Releases page](https://github.com/bhaskarjha-com/niyantra/releases).

### Build from Source

```bash
git clone https://github.com/bhaskarjha-com/niyantra.git
cd niyantra

# macOS/Linux (with make)
make build

# Windows (or without make)
go build -o niyantra.exe ./cmd/niyantra
```

### Docker

```bash
# Quick start with docker-compose
git clone https://github.com/bhaskarjha-com/niyantra.git
cd niyantra
docker compose up -d
# Dashboard at http://localhost:9222
```

Or run directly:

```bash
docker build -t niyantra:latest .
docker run -p 9222:9222 -v ./niyantra-data:/data niyantra:latest
```

Two image variants: **distroless** (default, ~15 MB, no shell) and **alpine** (`--target runtime-shell`, includes shell for `docker exec`).

## Try It Now

Don't have Antigravity running? No problem. The `demo` command seeds realistic sample data so you can explore immediately:

```bash
# macOS/Linux
make demo     # Seeds data + launches serve

# Windows
go run ./cmd/niyantra demo
go run ./cmd/niyantra serve
```

When you're ready to use real data:

```bash
niyantra snap     # Capture your Antigravity account's quota (Antigravity must be running)
niyantra serve    # Dashboard shows your real data
```

---

## Who Is This For?

- **Multi-account developers** juggling work + personal AI accounts
- **AI power users** paying $100-500/month across multiple subscriptions
- **Privacy-conscious developers** who want local-only data, no cloud dashboards
- **Freelancers & contractors** tracking AI costs per client or project
- **Anyone tired of checking 5 different dashboards** to answer "am I ready to code?"

---

## Features

### Know Your Quotas

Auto-capture Antigravity per-model quotas (Claude, Gemini, GPT) with rolling 5-hour reset detection. Track your native **Google AI Credits** balances continuously. Monitor **Codex/ChatGPT** via OAuth API, track **Claude Code** rate limits + deep JSONL token analytics, track **Cursor** usage via session token API, monitor **Gemini CLI** quota via OAuth, and track **GitHub Copilot** premium interactions via PAT. **Quick Adjust** lets you fine-tune stale values with ±5%/±10% buttons right on the dashboard. **7 providers** in one unified view. **Plugin system** (F18) lets you track any custom AI service via external scripts in any language.

### Control Your Spending

Track subscriptions across **26+ AI platforms** with renewals, spending breakdowns, and CSV export. Set a monthly budget and get forecasts before you overspend. Visual renewal calendar so nothing surprises you. **Estimated cost tracking** calculates per-model spend from quota deltas × model pricing. **Git commit correlation** attributes AI costs to specific feature branches.

### Let AI Help You Code Smarter

**Switch Advisor** ranks your accounts and tells you which one to use right now. **MCP Server** (12 tools over stdio + Streamable HTTP) lets your AI agent check quotas, analyze spending, query plugin data, and get routing recommendations mid-task — locally or remotely.

### Stay Informed

**Quad-channel notifications** alert you when quotas drop below threshold:
- **OS-native** (PowerShell / osascript / notify-send)
- **SMTP Email** (TLS/STARTTLS, HTML templates)
- **Webhooks** (Discord, Telegram, Slack, ntfy/Gotify)
- **WebPush** (browser push, works with tab closed)

### Your Data, Your Machine

SQLite database. No cloud. No telemetry. Full provenance audit trail on every snapshot -- you can always verify *how* and *when* data entered the system.

---

## Dashboard

**4 tabs** -- Quotas, Subscriptions, Overview, Settings

| Tab | What it shows |
|-----|---------------|
| **Quotas** | Provider-sectioned layout (Antigravity/Codex/Claude/Cursor/Gemini/Copilot), per-model progress bars with reset timers, Quick Adjust (±5%/±10%), provider, status & tag filters, split-button snap, twin-axis history chart, activity heatmap, AI Credits tracking |
| **Subscriptions** | Hybrid card + provider layout with spend summary, search, 26 platform presets, CSV export |
| **Overview** | Monthly budget vs actual, switch advisor, provider health cards, estimated cost tracking, Git commit costs, token usage analytics, sessions timeline, renewal calendar, JSON/CSV export |
| **Settings** | Auto-capture (7 providers), polling interval, notifications (4 channels), plugin management, model pricing, Claude bridge, backup/restore, command palette (`Ctrl+K`) |

---

## CLI Reference

| Command | What it does |
|---------|-------------|
| `niyantra snap` | Capture current Antigravity account's quota |
| `niyantra status` | Show all accounts' readiness (offline) |
| `niyantra serve` | Launch web dashboard at `localhost:9222` |
| `niyantra mcp` | Start MCP server (stdio) for AI agents |
| `niyantra demo` | Seed database with sample data |
| `niyantra backup` | Create timestamped database backup |
| `niyantra restore <file>` | Restore from backup |
| `niyantra healthcheck` | Docker health probe (GET /healthz) |
| `niyantra version` | Print version |

**Flags:** `--port 9222` `--bind 127.0.0.1` `--db ~/.niyantra/niyantra.db` `--auth user:pass` `--debug`

**Environment Variables:** `NIYANTRA_PORT` `NIYANTRA_BIND` `NIYANTRA_DB` `NIYANTRA_AUTH` (CLI flags take precedence)

## MCP Integration

Niyantra exposes quota intelligence to AI coding agents via the [Model Context Protocol](https://modelcontextprotocol.io).

**12 tools:** `quota_status` `model_availability` `usage_intelligence` `budget_forecast` `best_model` `analyze_spending` `switch_recommendation` `codex_status` `quota_forecast` `token_usage` `git_commit_costs` `plugin_status`

**Stdio transport** (local agents):
```json
{
  "mcpServers": {
    "niyantra": {
      "command": "niyantra",
      "args": ["mcp"]
    }
  }
}
```

**Streamable HTTP transport** (remote agents): `POST /mcp` on the running dashboard server. SSE streaming, session management.

Then ask: *"What's my quota?"* or *"Which account should I use?"* or *"How much did my last feature branch cost?"*

---

## How Does Niyantra Compare?

| Feature | Niyantra | Quota trackers (onWatch, CodexBar) | Sub trackers (Wallos) | Account managers (Antigravity-Manager) |
|---------|----------|-------------------------------|---------------------------|-----------|
| AI quota monitoring | 7 providers (AG + Codex + Claude + Cursor + Gemini + Copilot + Manual) | 9-16+ providers | — | 4+ providers |
| Multi-account per provider | ✅ 28+ (passive, safe) | ❌ Single-account | — | ✅ (active switching ⚠️) |
| Subscription management | 26 AI platforms, renewals, CSV | — | Generic subs | — |
| Budget forecasting | Monthly budget with projections | — | Basic budget | — |
| Switch advisor (account routing) | Multi-factor scoring engine | — | — | — |
| MCP for AI agents | 12 tools over stdio + HTTP | — | — | — |
| Notifications | Quad-channel (OS + SMTP + Webhook + WebPush) | — | 10+ channels | — |
| Token analytics | Per-model cost estimation + git correlation | — | — | — |
| Renewal calendar | Visual month view | — | Calendar view | — |
| Activity audit trail | Full provenance on every data point | — | — | — |
| Zero-daemon default | Manual mode, opt-in auto | Daemon by default | N/A | Daemon |
| Account switching | ❌ (by design — safety) | ❌ | — | ✅ (⚠️ T&S risk) |
| Docker support | ✅ Distroless + Alpine | ✅ | ✅ | — |
| License | MIT | MIT / GPL-3 | GPL-3 | MIT |

> **Competitive position:** Niyantra scores **37+ features** in our competitive analysis — leading all 28 tools evaluated across 8 market categories. The closest competitor (onWatch) scores 12/37. See [VISION.md](docs/VISION.md) for full market positioning.

---

## FAQ / Troubleshooting

### "Antigravity not detected"

Niyantra detects the Antigravity language server process. Make sure:
1. Antigravity IDE is running (not just installed)
2. A workspace/file is open (the language server starts when you open a project)
3. Try `niyantra snap --debug` for detailed detection output

### "Can I use this without Antigravity?"

Yes! Niyantra's subscription manager, budget tracking, and renewal calendar work independently. Use `niyantra demo` to explore, or add subscriptions manually without any Antigravity integration.

### "Is my data sent anywhere?"

No. By default, Niyantra makes HTTP calls purely locally to `127.0.0.1` (your local Antigravity Language Server). There is no telemetry or analytics. **Optional provider polling** (Codex, Cursor, Gemini, Copilot) makes HTTPS calls to their respective APIs when explicitly enabled. **Optional notifications** (SMTP, Webhook, WebPush) make outbound calls to configured endpoints. See [SECURITY.md](docs/SECURITY.md) for the full threat model.

### "How do I update?"

Re-run the install command or download the latest from [Releases](https://github.com/bhaskarjha-com/niyantra/releases). Your data (`~/.niyantra/niyantra.db`) is preserved across updates.

---

## Dependencies

| Dependency | Purpose |
|-----------|---------|
| [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) | Pure-Go SQLite -- no CGo, true single binary |
| [`go-sdk/mcp`](https://github.com/modelcontextprotocol/go-sdk) | Official MCP Go SDK for AI agent integration |
| Go stdlib | Everything else -- HTTP, JSON, embed, crypto |

No web frameworks. No ORMs. Chart.js bundled locally from embedded assets.
Frontend: 30 TypeScript modules (strict mode) bundled by esbuild into a single IIFE.

## Documentation

| Document | Content |
|----------|---------|
| **[USER_GUIDE.md](docs/USER_GUIDE.md)** | **Complete feature guide — start here** |
| [VISION.md](docs/VISION.md) | Product vision, market position, roadmap (Phases 1-16), competitive analysis |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | System design, data flow, security model |
| [API_SPEC.md](docs/API_SPEC.md) | REST API reference (60 endpoints) |
| [DATA_MODEL.md](docs/DATA_MODEL.md) | SQLite schema v19 (19 tables) |
| [SECURITY.md](docs/SECURITY.md) | What data is accessed, network behavior, threat model |
| [TESTING.md](docs/TESTING.md) | 158 automated tests + manual test cases |
| [CONTRIBUTING.md](docs/CONTRIBUTING.md) | Development setup, code style, PR guidelines |
| [CHANGELOG.md](CHANGELOG.md) | Version history (v0.1.0 → v0.26.0) |

## Contributing

See [CONTRIBUTING.md](docs/CONTRIBUTING.md) for development setup. Quick start:

```bash
git clone https://github.com/bhaskarjha-com/niyantra.git
cd niyantra
make build          # Build binary
make test           # Run tests
niyantra demo       # Seed sample data
niyantra serve      # Launch dashboard for development
```

## License

[MIT](LICENSE) -- (c) 2026 Bhaskar Jha
