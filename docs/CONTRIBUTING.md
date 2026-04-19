# Contributing to Niyantra

## Build from Source

```bash
# Clone
git clone <repo-url>
cd niyantra

# Build (single binary, no external deps)
go build -o niyantra ./cmd/niyantra

# Verify
./niyantra version
```

### Requirements

- **Go 1.22+** — the only build dependency
- No CGo, no C compiler needed (`modernc.org/sqlite` is pure Go)
- No npm, no Node.js, no frontend build step (JS/CSS are embedded as-is)

## Run Locally

```bash
# Start the dashboard
./niyantra serve

# Open http://localhost:9222
# Click "Snap Now" to capture current Antigravity account
```

### Prerequisites

1. **Antigravity IDE must be running** — Niyantra talks to the language server the IDE starts
2. **An account must be logged in** — Niyantra captures whichever account is active

## Project Layout

```
cmd/niyantra/main.go         ← CLI entrypoint (snap, status, serve, version)

internal/
  client/                     ← Language server detection + API call
    client.go                    Detect() + FetchQuotas() — the only external call
    detect_windows.go            Windows: CIM → PowerShell → WMIC fallback chain
    detect_unix.go               macOS/Linux: ps aux
    ports.go                     Port discovery via lsof/ss/netstat
    probe.go                     Connect RPC endpoint validation
    types.go                     API response structs (UserStatusResponse, etc.)
    helpers.go                   Model grouping logic (claude_gpt / gemini_pro / gemini_flash)

  store/                      ← SQLite persistence
    store.go                     Open, migrate schema, close
    snapshots.go                 InsertSnapshot, LatestPerAccount, History
    accounts.go                  GetOrCreateAccount (upsert by email)

  readiness/                  ← Pure computation, zero I/O
    readiness.go                 Calculate() — groups models, computes percentages + countdowns

  web/                        ← HTTP server + embedded dashboard
    server.go                    Setup, handlers: GET /api/status, POST /api/snap
    static/                      Embedded via Go embed.FS
      index.html                  Single-page dashboard shell
      style.css                   Design system (CSS variables, dark/light themes)
      app.js                      Dashboard logic (vanilla JS, no frameworks)
```

## Key Design Decisions (read these first!)

### 1. No `hidden` attribute for toggles — use CSS classes

The HTML `hidden` attribute sets `display: none`, but any CSS rule with `display: flex/grid` silently overrides it. Use `.is-expanded` / `.is-collapsed` CSS classes instead.

```css
/* ✅ Correct */
.model-details { display: none; }
.model-details.is-expanded { display: flex; }

/* ❌ Wrong — display:flex overrides [hidden] */
.model-details { display: flex; }  /* hidden attr ignored! */
```

### 2. Serialize durations as seconds, not Go nanoseconds

Go's `time.Duration` marshals to JSON as **nanoseconds** (int64). This creates confusing numbers in the frontend. Always convert to seconds at the Go layer:

```go
// ✅ Correct
TimeUntilResetSec float64 `json:"timeUntilResetSec"`

// ❌ Wrong — JS receives nanoseconds like 16200000000000
TimeUntilReset time.Duration `json:"timeUntilReset"`
```

### 3. LatestPerAccount uses MAX(id), not MAX(captured_at)

If two snapshots have the same timestamp (same-second rapid clicks), `MAX(captured_at)` returns both. `MAX(id)` is always unique.

### 4. No auto-polling

The dashboard does NOT auto-refresh. Data updates only on manual snap or page reload. This matches the "zero daemon" philosophy.

### 5. Event delegation for dynamic content

Since `renderAccounts()` rebuilds `innerHTML`, inline `onclick` handlers can fail. Use event delegation on the grid container:

```js
grid.addEventListener('click', function(e) {
  var row = e.target.closest('.account-row[data-toggle]');
  if (!row) return;
  // ... toggle logic
});
```

## Testing

```bash
# Build
go build -o niyantra ./cmd/niyantra

# Run vet
go vet ./...

# Manual test: capture + view
./niyantra snap
./niyantra status
./niyantra serve
```

No automated test suite yet. Verification is manual: build succeeds, `snap` captures, dashboard renders, expand/collapse works.

## Common Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| "language server process not found" | Antigravity IDE not running | Start the IDE first |
| "not authenticated" | No account logged in | Log into Antigravity in the IDE |
| Dashboard shows stale data | No auto-refresh by design | Click "Snap Now" or reload |
| "AI Credits" missing | Not in local LS API | Cannot be fixed — cloud-only data |
| Binary won't cross-compile | `modernc.org/sqlite` needs correct GOOS/GOARCH | Set env vars explicitly |
