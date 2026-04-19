# Niyantra

**Local-first, multi-account quota ledger for Antigravity.**

Niyantra captures point-in-time quota snapshots on demand — no background polling, no daemons, no continuous API calls. You snap before you switch accounts, and Niyantra tells you which account is ready to use.

## Why

If you use Antigravity across multiple accounts (work, personal, client projects), you face a daily question: *"Which account has quota right now?"* Checking each one manually means logging in, waiting, switching — wasted time.

Niyantra solves this with one command:

```
niyantra snap       # captures current account's quota (1 API call)
niyantra status     # shows all accounts' readiness (0 API calls)
```

## Principles

- **Zero daemon** — no background process, no polling
- **One API call per snap** — minimal upstream footprint
- **Local-first** — all readiness computation from SQLite, no network
- **Single binary** — Go with embedded web assets, nothing to install
- **Multi-platform** — Windows, macOS, Linux

## Quick Start

```bash
# Build
go build -o niyantra ./cmd/niyantra

# Snap current account
./niyantra snap

# Check all accounts
./niyantra status

# Launch dashboard
./niyantra serve
# → http://localhost:9222
```

## Dashboard

The web dashboard shows a real-time readiness grid across all your accounts:

| Account | Claude+GPT | Gemini Pro | Gemini Flash | Status |
|---------|-----------|-----------|-------------|--------|
| work@company.com | 85% ↻3h | 100% ↻4h | 100% ↻4h | ✅ Ready |
| personal@gmail.com | 0% ↻1h | 45% ↻2h | 100% ↻4h | ⚠️ Partial |

Click **Snap Now** to capture the currently logged-in account's quota from the dashboard.

## Architecture

```
niyantra snap
    │
    ▼
┌─────────────────────┐
│  Antigravity Client  │  ← auto-detects local language server
│  (1 API call)        │  ← extracts CSRF token from process
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  SQLite Ledger       │  ← snapshots table + accounts table
│  (~/.niyantra/data)  │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Readiness Engine    │  ← pure local computation
│  (0 network calls)  │  ← reset countdown, staleness
└─────────────────────┘
```

## Project Structure

```
niyantra/
├── cmd/niyantra/          # CLI entrypoint
│   └── main.go
├── internal/
│   ├── client/            # Antigravity API client (process detection + fetch)
│   ├── store/             # SQLite storage (snapshots, accounts)
│   ├── readiness/         # Local readiness computation engine
│   └── web/               # HTTP server + embedded dashboard
│       ├── handlers.go
│       └── static/        # HTML, CSS, JS (embedded)
├── docs/                  # Project documentation
├── go.mod
├── go.sum
└── README.md
```

## License

Private. Not for redistribution.
