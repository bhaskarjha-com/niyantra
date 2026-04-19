# User Guide

A complete guide to using every Niyantra feature.

## Table of Contents

- [Getting Started](#getting-started)
- [Quota Tracking](#quota-tracking)
- [Dashboard](#dashboard)
- [Subscription Manager](#subscription-manager)
- [Budget & Forecasting](#budget--forecasting)
- [Auto-Capture](#auto-capture)
- [Switch Advisor](#switch-advisor)
- [MCP Server (AI Agent Integration)](#mcp-server)
- [Codex/ChatGPT Integration](#codexchatgpt-integration)
- [Claude Code Bridge](#claude-code-bridge)
- [Notifications](#notifications)
- [Command Palette](#command-palette)
- [Data Management](#data-management)
- [CLI Reference](#cli-reference)
- [Configuration](#configuration)

---

## Getting Started

### First Run (with sample data)

No setup required. Explore all features immediately:

```bash
niyantra demo     # Seeds 2 accounts, 24 snapshots, 5 subscriptions
niyantra serve    # Open http://localhost:9222
```

### First Run (with real data)

1. Make sure Antigravity IDE is running with a project open
2. Run your first snapshot:

```bash
niyantra snap     # Captures your account's quota (1 API call to localhost)
niyantra serve    # Dashboard at http://localhost:9222
```

You should see your account appear in the Quotas tab with per-model progress bars.

---

## Quota Tracking

### What Gets Captured

Each snapshot records:
- **Account email** and plan name
- **Per-model quotas**: remaining percentage, reset time, and exhaustion status
- **Prompt credits**: available and monthly totals
- **Provenance**: how, when, and where the data was captured

### Models Tracked

Antigravity exposes per-model quotas. Niyantra groups them into 3 logical pools:

| Group | Models | Color |
|-------|--------|-------|
| **Claude + GPT** | Claude Sonnet, GPT-4.1, etc. | Orange |
| **Gemini Pro** | Gemini 2.5 Pro | Green |
| **Gemini Flash** | Gemini 2.5 Flash | Blue |

Each pool has independent 5-hour rolling quotas that reset independently.

### Manual Snap

```bash
niyantra snap              # Capture current account
niyantra snap --debug      # Verbose output (useful for troubleshooting detection)
```

Or click **Snap Now** in the dashboard's Quotas tab.

### Status Check (offline)

```bash
niyantra status            # Shows all accounts, no network calls
```

Output shows a readiness grid: which accounts are ready, which are exhausted, when resets happen.

---

## Dashboard

Launch with `niyantra serve` and open `http://localhost:9222`.

### Quotas Tab

The default view showing all tracked Antigravity accounts.

**Readiness grid**: Each row is an account. Color-coded status:
- **Green (Ready)**: All quota pools above threshold
- **Yellow (Low)**: Some pools depleted
- **Red (Exhausted)**: Account cannot be used

**Click any row** to expand and see:
- Per-model progress bars with exact percentages
- Reset countdowns (e.g., "resets in 2h 15m")
- Prompt credits badge
- **Clear Snapshots** button — deletes all quota history for the account (keeps the account)
- **Remove Account** button — permanently deletes the account and all associated data

**Quota History Chart**: Below the grid. Shows quota trends over time as a line chart.
- Filter by account using the dropdown
- Change the number of displayed snapshots (20, 50, 100)
- Adapts to dark/light theme automatically

### Subscriptions Tab

Card-based view of all your AI subscriptions.

**Adding a subscription:**
1. Click **+ Add Subscription**
2. Choose from 26 platform presets (Antigravity, Claude, ChatGPT, Cursor, Copilot, Midjourney, etc.) or create a custom entry
3. Fill in plan, cost, billing cycle, renewal date
4. Click Save

**Each card shows:**
- Platform name, plan, and monthly cost
- Status badge (Active, Trial, Cancelled, Paused)
- Next renewal date
- Dashboard URL link (click to go to the provider's billing page)

**Search**: Type in the search box to filter subscriptions by name, plan, or email.

**CSV Export**: Click the export button to download all subscriptions as a CSV file for expense reports.

### Overview Tab

The intelligence hub combining data from all sources.

**Budget & Forecast section:**
- Monthly budget vs actual spending (set budget in Settings)
- Projected end-of-month spend based on current burn rate
- Days remaining in billing period

**Switch Advisor:**
- Recommends which account to use right now
- Actions: "switch" (use a different account), "stay" (current is best), "wait" (all exhausted, reset coming soon)
- Shows score breakdown: remaining% (60% weight), burn rate (20%), reset time (20%)

**Codex Status** (if configured):
- Shows Codex/ChatGPT quota across 5-hour, 7-day, and code review windows

**Sessions Timeline:**
- Shows detected usage sessions with duration, provider, and snapshot count

**Renewal Calendar:**
- Visual month-view grid with pins on renewal dates
- Navigate months with arrow buttons
- Legend shows which subscriptions renew when

### Settings Tab

**Capture Settings:**
- Auto-capture toggle (enable/disable background polling)
- Polling interval slider (30s to 300s)
- Manual snap button

**Budget:**
- Monthly budget amount (used for forecasting on Overview tab)

**Notifications:**
- Enable/disable OS-native notifications
- Threshold slider (notify when any quota drops below X%)
- Test notification button

**Claude Code Bridge:**
- Enable/disable Claude Code rate limit monitoring
- Shows bridge status and latest rate limit data

**Codex/ChatGPT:**
- Enable/disable Codex API polling
- Shows detection status and credentials location

**Data Management:**
- Backup: Download a copy of your database
- Restore: Upload a backup file
- JSON Export: Download all data as JSON
- JSON Import: Upload and merge data from another Niyantra instance

**Account Management** (via Quotas tab):
- Expand any account row → **Clear Snapshots** or **Remove Account**
- Both actions require confirmation and are logged to the activity log

---

## Subscription Manager

### 26 Platform Presets

When adding a subscription, choose from built-in presets:

| Category | Platforms |
|----------|-----------|
| **AI Coding** | Antigravity, Cursor, GitHub Copilot, Codex/ChatGPT |
| **AI Chat** | Claude, ChatGPT, Gemini, Perplexity |
| **AI API** | OpenAI API, Anthropic API, Google AI Studio |
| **AI Creative** | Midjourney, Runway, ElevenLabs, Suno, DALL-E |
| **AI Productivity** | Notion AI, Grammarly, Jasper |
| **AI DevTools** | Replit, Tabnine, Codeium, v0 |

Each preset pre-fills the platform name, category, and typical pricing. You can customize everything.

### Subscription Statuses

- **Active**: Currently paying
- **Trial**: Free trial (set trial end date to get renewal alerts)
- **Paused**: Temporarily suspended
- **Cancelled**: No longer paying

### Editing and Deleting

Click any subscription card to edit its details. Use the delete button to remove it.

---

## Budget & Forecasting

### Setting Your Budget

1. Go to **Settings** tab
2. Enter your monthly AI budget (e.g., $150)
3. The Overview tab will now show:
   - Current spend vs budget
   - Projected end-of-month spend
   - Warning if you're on track to exceed budget

### How Forecasting Works

Niyantra calculates your daily burn rate from active subscriptions and projects it across the remaining days in the month. This is a simple linear projection — it doesn't account for variable usage-based billing.

---

## Auto-Capture

### Enabling

1. Go to **Settings** tab
2. Toggle **Auto-Capture** to ON
3. Set polling interval (default: 60 seconds)

### How It Works

When enabled, Niyantra polls the Antigravity language server at your configured interval. Each poll:
1. Detects the running language server process
2. Makes one HTTP call to fetch quota data
3. Stores the snapshot with provenance tag `capture_method: auto`
4. Detects reset cycles (when quotas jump back up)
5. Updates session tracking
6. Triggers notifications if thresholds are breached

### Safety

- **Zero-daemon by default**: Auto-capture only runs when explicitly enabled AND `niyantra serve` is running
- **No background service**: Stops when you close the dashboard
- **One call per poll**: Each poll makes exactly 1 HTTP call to localhost
- **Exponential backoff**: If detection fails, wait time increases to avoid hammering the process list

---

## Switch Advisor

The switch advisor helps you choose which Antigravity account to use when you have multiple accounts.

### How Scoring Works

Each account gets a score (0-100) based on three factors:

| Factor | Weight | What it measures |
|--------|--------|-----------------|
| Remaining % | 60% | Average remaining quota across all model groups |
| Burn Rate | 20% | How fast you're consuming (from usage intelligence) |
| Reset Time | 20% | How soon the lowest quota resets |

### Actions

- **"switch"**: Another account scores significantly higher. Switch to it.
- **"stay"**: Current account is best (or close enough). Keep using it.
- **"wait"**: All accounts are exhausted, but one resets soon. Wait for it.

### Accessing

- **Dashboard**: Overview tab shows the advisor recommendation
- **CLI**: Data is shown in status output
- **MCP**: AI agents can call `switch_recommendation` tool

---

## MCP Server

Niyantra exposes 8 tools to AI coding agents via the [Model Context Protocol](https://modelcontextprotocol.io).

### Setup

Add to your MCP client config:

**Claude Desktop** (`~/.config/claude/claude_desktop_config.json`):
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

**Antigravity** (Settings > MCP): Add as a stdio MCP server pointing to your `niyantra` binary.

### Available Tools

| Tool | What you can ask |
|------|-----------------|
| `quota_status` | "What's my Antigravity quota?" |
| `model_availability` | "Is Claude Sonnet available?" |
| `usage_intelligence` | "How fast am I burning quota?" |
| `budget_forecast` | "Will I stay under budget this month?" |
| `best_model` | "Which model has the most quota?" |
| `analyze_spending` | "Break down my AI spending by category" |
| `switch_recommendation` | "Should I switch accounts?" |
| `codex_status` | "What's my Codex/ChatGPT status?" |

### Running

```bash
niyantra mcp    # Starts MCP server on stdio (meant for MCP clients, not direct use)
```

The MCP server reads from the same SQLite database as the dashboard. It makes **zero network calls** — all data comes from previously captured snapshots.

---

## Codex/ChatGPT Integration

### How It Works

Niyantra can poll the Codex/ChatGPT API to track multi-window quotas:
- **5-hour window**: Rolling quota for recent usage
- **7-day window**: Weekly quota limit
- **Code review**: Separate quota for code review features

### Setup

1. Niyantra auto-detects credentials from `~/.codex/auth.json`
2. Enable in **Settings** tab > Codex section
3. When auto-capture is on, Codex is polled alongside Antigravity

### Dashboard

The **Overview** tab shows a Codex status card with all three quota windows and their current usage.

---

## Claude Code Bridge

### What It Does

Monitors Claude Code's rate limit data via a statusline bridge. This patches Claude Code's settings to expose rate limit information that Niyantra can read.

### Setup

1. Enable in **Settings** tab > Claude Code Bridge
2. Niyantra patches `~/.claude/settings.json` to add statusline data
3. Rate limit data appears in the dashboard

### What Gets Tracked

- 5-hour rate limit usage
- 7-day rate limit usage
- Current status (healthy, warning, throttled)

---

## Notifications

### OS-Native Alerts

Niyantra sends native OS notifications when quotas drop below your configured threshold.

| OS | Method |
|----|--------|
| Windows | PowerShell toast notification |
| macOS | `osascript` notification |
| Linux | `notify-send` |

### Configuration

1. **Settings** tab > Notifications
2. Toggle ON
3. Set threshold (e.g., 20% — notify when any quota drops below 20%)
4. Click **Test** to verify notifications work

### Once-Per-Cycle Guard

Notifications fire once per reset cycle, not every poll. This prevents notification spam while ensuring you don't miss important alerts.

---

## Command Palette

Press **Ctrl+K** (or **Cmd+K** on Mac) anywhere in the dashboard to open the command palette.

### Available Commands

- Snap Now, Toggle Auto-Capture, Switch Theme
- Export CSV, Export JSON, Import JSON, Download Backup
- Navigate tabs (Quotas, Subscriptions, Overview, Settings)
- Add Subscription, Test Notification

### Fuzzy Search

Type to filter commands. The palette uses fuzzy matching, so typing "exp" matches "Export CSV", "Export JSON", etc.

---

## Data Management

### Backup

```bash
niyantra backup                              # Creates timestamped backup
niyantra backup --db ~/.niyantra/niyantra.db # Specify database
```

Or use the **Settings** tab > Download Backup button.

Backups are saved as `niyantra-backup-YYYYMMDD-HHMMSS.db` in the current directory.

### Restore

```bash
niyantra restore ./niyantra-backup-20260419-120000.db
```

The restore command validates the schema before replacing your database.

### JSON Export

**Settings** tab > Export JSON — downloads all data (accounts, subscriptions, snapshots, config, activity log) as a single JSON file.

### JSON Import

**Settings** tab > Import JSON — uploads a JSON file and merges data with additive deduplication:
- Accounts are matched by email (no duplicates)
- Subscriptions are matched by platform + email
- Snapshots are matched by account + timestamp
- Existing data is never overwritten or deleted

### Clear Snapshots

To clear all quota history for a specific account:
1. Go to **Quotas** tab
2. Click the account row to expand it
3. Click **Clear Snapshots**
4. Confirm in the dialog

The account itself remains — only snapshot history is deleted.

### Remove Account

To permanently delete a tracked account and all its data:
1. Go to **Quotas** tab
2. Click the account row to expand it
3. Click **Remove Account**
4. Confirm in the dialog

This cascade-deletes the account, all snapshots, reset cycles, and codex snapshots. The next time you `snap` with this email, it will be re-created as a fresh account.

---

## CLI Reference

### Commands

```bash
niyantra snap                    # Capture Antigravity quota
niyantra status                  # Show all accounts (no network)
niyantra serve                   # Launch dashboard
niyantra serve --port 8080       # Custom port
niyantra serve --auth admin:pass # Password-protect dashboard
niyantra mcp                     # Start MCP server (stdio)
niyantra demo                    # Seed sample data
niyantra backup                  # Backup database
niyantra restore <file>          # Restore from backup
niyantra version                 # Print version
```

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | `~/.niyantra/niyantra.db` | Database file path |
| `--debug` | false | Enable verbose logging |
| `--port` | 9222 | Dashboard port (serve only) |
| `--auth` | (none) | HTTP basic auth as `user:pass` (serve only) |

---

## Configuration

All configuration is stored in the SQLite database (not config files). Change settings via the dashboard's Settings tab.

| Setting | Default | What it does |
|---------|---------|-------------|
| `auto_capture` | false | Enable background polling |
| `poll_interval` | 60 | Seconds between auto-capture polls |
| `notify_enabled` | false | Enable OS notifications |
| `notify_threshold` | 20 | Notify when quota drops below this % |
| `budget_monthly` | 0 | Monthly AI budget for forecasting |
| `claude_bridge` | false | Enable Claude Code statusline bridge |
| `codex_capture` | false | Enable Codex/ChatGPT polling |
| `session_idle_timeout` | 300 | Seconds of inactivity before ending a session |
| `auto_link_subs` | true | Auto-create subscription when snapping a new account |
| `retention_days` | 90 | Days to keep old snapshots (0 = keep forever) |

---

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` / `Cmd+K` | Open command palette |
| `1` | Switch to Quotas tab |
| `2` | Switch to Subscriptions tab |
| `3` | Switch to Overview tab |
| `4` | Switch to Settings tab |

---

## Theme

Niyantra detects your system theme preference (dark/light) on first visit and applies it automatically. Toggle manually with the sun/moon icon in the header. Your preference is saved across sessions.
