# API Specification: Niyantra

## Base URL

```
http://localhost:9222
```

## Authentication

Optional. If `--auth user:pass` is provided at startup, all endpoints require HTTP Basic Auth.

---

## Endpoints

### `GET /`

Serves the single-page dashboard (embedded HTML/CSS/JS).

**Response:** `text/html`

---

### `GET /api/status`

Returns the readiness state of all tracked accounts. **Zero network calls** — reads only from local SQLite.

**Response:** `200 OK`

```json
{
  "accounts": [
    {
      "accountId": 1,
      "latestSnapshotId": 308,
      "email": "work@company.com",
      "planName": "Pro",
      "lastSeen": "2026-04-17T00:30:00Z",
      "stalenessLabel": "21 min ago",
      "isReady": true,
      "promptCredits": 500,
      "monthlyCredits": 50000,
      "groups": [
        {
          "groupKey": "claude_gpt",
          "displayName": "Claude + GPT",
          "remainingPercent": 40.0,
          "isExhausted": false,
          "isReady": true,
          "color": "#D97757",
          "resetTime": "2026-04-17T04:24:00Z",
          "timeUntilResetSec": 13800.5
        },
        {
          "groupKey": "gemini_pro",
          "displayName": "Gemini Pro",
          "remainingPercent": 100.0,
          "isExhausted": false,
          "isReady": true,
          "color": "#10B981",
          "resetTime": "2026-04-17T04:48:00Z",
          "timeUntilResetSec": 15240.0
        },
        {
          "groupKey": "gemini_flash",
          "displayName": "Gemini Flash",
          "remainingPercent": 100.0,
          "isExhausted": false,
          "isReady": true,
          "color": "#3B82F6",
          "resetTime": "2026-04-17T04:48:00Z",
          "timeUntilResetSec": 15240.0
        }
      ],
      "models": [
        {
          "modelId": "MODEL_PLACEHOLDER_M35",
          "label": "Claude Sonnet 4.6 (Thinking)",
          "remainingPercent": 40.0,
          "isExhausted": false,
          "resetSeconds": 13800.5,
          "groupKey": "claude_gpt"
        },
        {
          "modelId": "MODEL_PLACEHOLDER_M37",
          "label": "Gemini 3.1 Pro (High)",
          "remainingPercent": 100.0,
          "isExhausted": false,
          "resetSeconds": 15240.0,
          "groupKey": "gemini_pro"
        }
      ]
    }
  ],
  "snapshotCount": 12,
  "accountCount": 1
}
```

**Field Reference — Account:**

| Field | Type | Description |
|-------|------|-------------|
| `accountId` | `int64` | Internal account identifier |
| `latestSnapshotId` | `int64` | ID of the most recent snapshot (used by Quick Adjust) |
| `email` | `string` | Antigravity account email |
| `planName` | `string` | Subscription plan (Free, Pro, Enterprise) |
| `notes` | `string` | User-defined note for this account (Phase 13 F1) |
| `tags` | `string` | Comma-separated tags (e.g., `"work,primary"`) (Phase 13 F1) |
| `pinnedGroup` | `string` | Pinned quota group key for this account (Phase 13 F3) |
| `creditRenewalDay` | `int` | Day of month (1-31) when AI credits refresh. 0 = not set. |
| `lastSeen` | `string` | ISO 8601 timestamp of last snapshot |
| `stalenessLabel` | `string` | Human-readable age ("just now", "3 min ago") |
| `isReady` | `bool` | `true` if ALL groups have remaining > 0 |
| `promptCredits` | `float64` | Remaining monthly prompt credits |
| `monthlyCredits` | `int` | Total monthly prompt credit allocation |

**Field Reference — Group:**

| Field | Type | Description |
|-------|------|-------------|
| `groupKey` | `string` | One of: `claude_gpt`, `gemini_pro`, `gemini_flash` |
| `displayName` | `string` | Human-readable group name |
| `remainingPercent` | `float64` | 0–100, average across models in this group |
| `isExhausted` | `bool` | `true` if any model in group has 0% remaining |
| `timeUntilResetSec` | `float64` | Seconds until the group's quota resets |
| `color` | `string` | Hex color for UI rendering |

**Field Reference — Model (per-model detail):**

| Field | Type | Description |
|-------|------|-------------|
| `modelId` | `string` | Internal model identifier |
| `label` | `string` | Display name (e.g., "Claude Sonnet 4.6 (Thinking)") |
| `remainingPercent` | `float64` | 0–100, individual model's remaining quota |
| `isExhausted` | `bool` | `true` if model has 0% remaining |
| `resetSeconds` | `float64` | Seconds until this model's quota resets |
| `groupKey` | `string` | Which group this model belongs to |

---

### `POST /api/snap`

Triggers an on-demand snapshot of the currently logged-in Antigravity account. **One upstream API call** to the local language server.

**Request:** No body required.

**Response (success):** `200 OK`

```json
{
  "message": "snapshot captured",
  "email": "work@company.com",
  "planName": "Pro",
  "snapshotId": 12,
  "accountId": 1,
  "accountCount": 1,
  "snapshotCount": 12,
  "accounts": [
    // ... same structure as GET /api/status, refreshed after capture
  ]
}
```

**Response (language server not found):** `502 Bad Gateway`

```json
{
  "error": "antigravity: language server process not found"
}
```

**Response (not authenticated):** `502 Bad Gateway`

```json
{
  "error": "antigravity: not authenticated"
}
```

---

### `PATCH /api/snap/adjust` (Quick Adjust)

Fine-tune model quota percentages on an existing snapshot. Designed for manual correction when LS cache data is slightly stale (~60-120s).

**Request:** `application/json`

```json
{
  "snapshotId": 308,
  "adjustments": [
    { "label": "Claude Sonnet 4.6", "remainingPercent": 15 },
    { "label": "Gemini 3.1 Pro (High)", "remainingPercent": 80 }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `snapshotId` | `int64` | Target snapshot to adjust (from `latestSnapshotId`) |
| `adjustments[].label` | `string` | Model display label to match |
| `adjustments[].remainingPercent` | `float64` | New remaining percentage (clamped 0-100) |

**Response (success):** `200 OK`

```json
{
  "message": "snapshot adjusted",
  "snapshotId": 308,
  "adjustments": 2,
  "models": [
    { "modelId": "...", "label": "Claude Sonnet 4.6", "remainingFraction": 0.15, "remainingPercent": 15, "isExhausted": false },
    { "modelId": "...", "label": "Gemini 3.1 Pro (High)", "remainingFraction": 0.80, "remainingPercent": 80, "isExhausted": false }
  ]
}
```

**Response (no matching models):** `400 Bad Request`

```json
{ "error": "no matching models found to adjust" }
```

---

### `GET /api/history`

Returns snapshot history for a specific account or all accounts.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `account` | `int64` | all | Filter by account ID |
| `limit` | `int` | 50 | Max snapshots to return |

**Response:** `200 OK`

```json
{
  "snapshots": [
    {
      "id": 12,
      "accountId": 1,
      "email": "work@company.com",
      "capturedAt": "2026-04-17T00:30:00Z",
      "planName": "Pro",
      "groups": [
        {
          "groupKey": "claude_gpt",
          "remainingPercent": 40.0,
          "isExhausted": false,
          "resetTime": "2026-04-17T04:24:00Z"
        }
      ]
    }
  ]
}
```

---

## Subscription Endpoints (Manual Tracking)

### `GET /api/subscriptions`

Lists all subscriptions. Supports `?status=active` and `?category=coding` filters.

**Response:** `200 OK` — `{ "subscriptions": [...], "count": N }`

Each subscription includes computed `daysUntilRenewal` and `daysUntilTrialEnd`.

### `POST /api/subscriptions`

Creates a subscription. Required field: `platform`.

**Response:** `201 Created` — returns the created subscription object.

### `GET /api/subscriptions/:id`

Returns one subscription. **Response:** `200` or `404`.

### `PUT /api/subscriptions/:id`

Updates a subscription. Same body as POST. **Response:** `200` or `404`.

### `DELETE /api/subscriptions/:id`

Deletes a subscription. **Response:** `200 OK` — `{ "message": "deleted" }`

### `GET /api/overview`

Unified stats: monthly/annual spend, category breakdown, upcoming renewals, quick links, auto-tracked quota summary.

### `GET /api/presets`

Returns 26 platform preset templates with pre-filled cost, limits, notes, dashboard URLs, and status page URLs.

### `GET /api/export/csv`

Downloads all subscriptions as CSV. Columns: Platform, Category, Plan, Status, Monthly Cost, Currency, Billing Cycle, Annual Cost, Email, Next Renewal, Notes, Dashboard URL.

---

## Error Format

All errors use a consistent JSON envelope:

```json
{
  "error": "human-readable error message"
}
```

HTTP status codes:
- `400` — Bad request (invalid parameters)
- `404` — Not found (subscription ID doesn't exist)
- `405` — Method not allowed
- `500` — Internal server error (database failure)
- `502` — Bad gateway (Antigravity language server unreachable)

## CORS

Not needed. The dashboard is served from the same origin as the API.

## Rate Limiting

None. The tool is single-user by design.

---

## Config & Infrastructure Endpoints (v3)

### `GET /api/config`

Returns all server configuration entries, grouped by category.

**Response:** `200 OK`

```json
{
  "config": [
    {
      "key": "auto_capture",
      "value": "false",
      "valueType": "bool",
      "category": "capture",
      "label": "Auto Capture",
      "description": "Enable autonomous data capture (polling, log parsing)"
    },
    {
      "key": "budget_monthly",
      "value": "200",
      "valueType": "float",
      "category": "display",
      "label": "Monthly Budget",
      "description": "Monthly AI spending budget ($)"
    }
  ]
}
```

### `PUT /api/config`

Updates a config entry. Validates value against `value_type`. Logs `config_change` event in activity log.

**Request:**

```json
{
  "key": "budget_monthly",
  "value": "250"
}
```

**Response:** `200 OK` — returns updated config entry.

**Validation rules:**

| Key | Valid Values |
|-----|------------|
| `auto_capture` | `true`, `false` |
| `poll_interval` | `30`–`3600` (integer) |
| `auto_link_subs` | `true`, `false` |
| `budget_monthly` | `0`+ (float) |
| `currency` | `USD`, `EUR`, `GBP`, `INR`, `CAD`, `AUD` |
| `retention_days` | `30`–`3650` (integer) |

### `GET /api/activity`

Returns recent activity log entries.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | `int` | 50 | Max entries to return |
| `type` | `string` | all | Filter by event type (e.g., `snap`, `config_change`, `quota_alert`) |

**Response:** `200 OK`

```json
{
  "entries": [
    {
      "id": 42,
      "timestamp": "2026-04-17T06:30:00Z",
      "level": "info",
      "source": "ui",
      "eventType": "snap",
      "accountEmail": "user@gmail.com",
      "snapshotId": 47,
      "details": "{\"plan\":\"Pro\",\"method\":\"manual\",\"source\":\"ui\"}"
    }
  ]
}
```

### `GET /api/mode`

Lightweight endpoint for the header mode badge. Returns current capture mode, agent polling status, and data source info.

**Response:** `200 OK`

```json
{
  "mode": "auto",
  "autoCapture": true,
  "isPolling": true,
  "pollInterval": 300,
  "lastPoll": "2026-04-17T14:30:00Z",
  "lastPollOK": true,
  "sources": [
    { "id": "antigravity", "name": "Antigravity", "enabled": true, "lastCapture": "2026-04-17T14:30:00Z", "captureCount": 47 },
    { "id": "claude_code", "name": "Claude Code", "enabled": false, "lastCapture": null, "captureCount": 0 },
    { "id": "codex", "name": "Codex", "enabled": false, "lastCapture": null, "captureCount": 0 }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `mode` | string | `"manual"` or `"auto"` — derived from config |
| `autoCapture` | bool | Whether auto-capture is enabled in config |
| `isPolling` | bool | Whether the polling agent goroutine is currently running |
| `pollInterval` | int | Configured polling interval in seconds |
| `lastPoll` | string? | ISO timestamp of last poll attempt (null if never polled) |
| `lastPollOK` | bool? | Whether the last poll succeeded (null if never polled) |
| `sources` | array | Data source registry with capture counts |

---

### `GET /api/usage`

Returns per-model usage intelligence and budget burn rate forecast. Requires at least 30 minutes of auto-capture data for rate/projection calculations.

**Query Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `account` | int | No | Filter by account ID. Defaults to first account. |

**Response:** `200 OK`

| Field | Type | Description |
|-------|------|-------------|
| `models` | array | Per-model usage summaries with intelligence data |
| `models[].modelId` | string | Opaque model identifier |
| `models[].label` | string | Human-readable model name |
| `models[].group` | string | Quota group key (claude_gpt, gemini_pro, gemini_flash) |
| `models[].remainingFraction` | float | Current remaining quota (0.0-1.0) |
| `models[].usagePercent` | float | Usage percentage (0-100) |
| `models[].isExhausted` | bool | Whether quota is depleted |
| `models[].resetTime` | string? | ISO timestamp of next reset |
| `models[].timeUntilReset` | string | Human-readable time until reset (e.g., "2h30m") |
| `models[].currentRate` | float | Usage fraction consumed per hour (0 if insufficient data) |
| `models[].projectedUsage` | float | Projected total usage at reset (0.0-1.0) |
| `models[].projectedExhaustion` | string? | ISO timestamp when quota will hit 0 at current rate |
| `models[].hasIntelligence` | bool | True if rate data is available (≥30 min of tracking) |
| `models[].completedCycles` | int | Number of completed reset cycles tracked |
| `models[].avgPerCycle` | float | Average usage delta per cycle |
| `models[].peakCycle` | float | Highest peak usage observed in any cycle |
| `models[].cycleAge` | string | How long the current cycle has been active |
| `models[].cycleSnapshots` | int | Number of snapshots in the current cycle |
| `budgetForecast` | object? | Budget projection (null if no budget set) |
| `budgetForecast.monthlyBudget` | float | Configured monthly budget |
| `budgetForecast.currentSpend` | float | Current month's spend from subscriptions |
| `budgetForecast.projectedMonthlySpend` | float | Projected spend at current burn rate |
| `budgetForecast.burnRate` | float | Dollars per day |
| `budgetForecast.daysUntilBudgetExhausted` | int? | When budget runs out (null if on track) |
| `budgetForecast.onTrack` | bool | Whether projected spend is within budget |

---

### `GET /api/forecast` (Phase 14)

Returns time-to-exhaustion (TTX) forecasts computed from sliding-window rate analysis of recent snapshot history (last 60 minutes). Provides per-group predictions for Antigravity accounts, Claude Code, and Codex.

**Response:** `200 OK`

```json
{
  "antigravity": [
    {
      "accountId": 1,
      "email": "user@company.com",
      "planName": "Pro",
      "groups": [
        {
          "groupKey": "claude_gpt",
          "displayName": "Claude + GPT",
          "burnRate": 0.15,
          "ttxHours": 2.5,
          "ttxLabel": "~2.5h",
          "remaining": 0.375,
          "confidence": "high",
          "willExhaust": false,
          "severity": "caution"
        }
      ]
    }
  ],
  "claude": {
    "windows": [
      { "window": "5-hour", "burnRate": 8.5, "ttxHours": 6.3, "ttxLabel": "~6.3h", "severity": "safe" },
      { "window": "7-day", "burnRate": 1.2, "ttxHours": 72, "ttxLabel": "~3d", "severity": "safe" }
    ]
  },
  "advisor": { ... }
}
```

**Field Reference — Group Forecast:**

| Field | Type | Description |
|-------|------|-------------|
| `groupKey` | string | Quota group identifier |
| `burnRate` | float | Fraction consumed per hour (0.0–1.0 scale) |
| `ttxHours` | float | Hours until exhaustion (-1 = no data, 0 = exhausted) |
| `ttxLabel` | string | Human-readable: "~2.5h", "~45m", "idle", "exhausted" |
| `remaining` | float | Current remaining fraction (0.0–1.0) |
| `confidence` | string | Data quality: "high" (≥6 points), "medium" (3–5), "low" (2), "none" |
| `willExhaust` | bool | Projected to exhaust before reset |
| `severity` | string | "safe" (>3h), "caution" (1–3h), "warning" (<1h), "critical" (<30m) |

> **Algorithm:** Uses recency-weighted sliding window over the last 60 minutes of snapshots. More recent data points are weighted ~2× heavier than older ones. Accounts for idle periods properly — unlike the older cycle-lifetime average which diluted during inactivity.

---

### `GET /api/cost` (Phase 14: F8)

Returns estimated dollar costs for all tracked accounts based on quota fraction consumption and configurable model pricing (from F5). Costs are computed by mapping Δ remainingFraction × quota ceiling × blended model price.

**Response:** `200 OK`

```json
{
  "accounts": [
    {
      "accountId": 1,
      "email": "user@company.com",
      "totalCost": 17.20,
      "totalLabel": "$17.20",
      "groups": [
        {
          "groupKey": "claude_gpt",
          "displayName": "Claude + GPT",
          "consumedFraction": 0.40,
          "estimatedTokens": 2000000,
          "estimatedCost": 17.20,
          "costPerHour": 4.30,
          "costLabel": "$17.20",
          "hourlyLabel": "$4.30/hr",
          "hasData": true
        }
      ]
    }
  ],
  "totalCost": 17.20,
  "totalLabel": "$17.20",
  "quotaCeilings": {
    "claude_gpt": { "groupKey": "claude_gpt", "displayName": "Claude + GPT", "tokensPerCycle": 5000000, "cycleDurationHours": 5 },
    "gemini_pro": { "groupKey": "gemini_pro", "displayName": "Gemini Pro", "tokensPerCycle": 3000000, "cycleDurationHours": 5 },
    "gemini_flash": { "groupKey": "gemini_flash", "displayName": "Gemini Flash", "tokensPerCycle": 10000000, "cycleDurationHours": 5 }
  }
}
```

**Field Reference — Group Cost:**

| Field | Type | Description |
|-------|------|-------------|
| `groupKey` | string | Quota group identifier |
| `consumedFraction` | float | Fraction consumed this cycle (1.0 - remaining) |
| `estimatedTokens` | float | Estimated tokens consumed (fraction × ceiling) |
| `estimatedCost` | float | Dollar cost estimate |
| `costPerHour` | float | Current hourly burn rate in dollars |
| `costLabel` | string | Formatted cost: "$17.20" |
| `hourlyLabel` | string | Formatted hourly rate: "$4.30/hr" |
| `hasData` | bool | True if burn rate / remaining data is available |

> **Algorithm:** `consumed_fraction × tokens_per_cycle × blended_price_per_token`, where blended price is 40% input + 60% output pricing (typical coding-assistant token split). Quota ceilings are configurable via Settings and default to 5M (Claude+GPT), 3M (Gemini Pro), 10M (Gemini Flash) tokens per 5-hour cycle.

> **Note:** `/api/status` also includes an `estimatedCosts` field (keyed by accountId) with the same per-account cost data for inline rendering in the Quotas grid.

---

## Error Format

All errors use a consistent JSON envelope:

```json
{
  "error": "human-readable error message"
}
```

HTTP status codes:
- `400` — Bad request (invalid parameters, invalid config value)
- `404` — Not found (subscription ID doesn't exist)
- `405` — Method not allowed
- `500` — Internal server error (database failure)
- `502` — Bad gateway (Antigravity language server unreachable)

## CORS

Not needed. The dashboard is served from the same origin as the API.

## Rate Limiting

None. The tool is single-user by design.

---

## Client-Side Features

### Theme Preference (localStorage)

The only setting stored in localStorage — it's a pure visual preference with zero server impact.

- **Storage key:** `niyantra-theme`
- **Value:** `dark`, `light`, or absent (system default)

> **Note:** Budget and currency are stored in the SQLite `config` table (accessible via `/api/config`) because they're needed server-side for CSV export, future MCP queries, and CLI reports.

### Smart Insights (Client-Side Computed)

Generated from `/api/overview` + `/api/subscriptions` data:
- Active subscription count
- Trial warnings
- Top spending category
- Imminent renewal alerts (≤3 days)
- Pay-as-you-go unbounded cost warnings
- Annual billing savings potential (~17%)

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `1` `2` `3` `4` | Switch tabs (Quotas/Subs/Overview/Settings) |
| `N` | Open new subscription modal |
| `S` | Trigger snap |
| `/` | Focus subscription search |
| `Esc` | Close any open modal |

### Subscription Search

- Real-time full-text filtering across all subscription card content
- Hides empty category labels when all cards in a category are filtered out

### PWA Manifest

- `manifest.json` served alongside dashboard assets
- Enables "Add to Home Screen" / "Install App" on supporting browsers
- `theme-color: #0a0e17` for native-feeling toolbar

---

## MCP Server (Phase 8)

Niyantra includes an MCP (Model Context Protocol) server that exposes quota intelligence to AI coding agents over stdio transport.

### Usage

```bash
niyantra mcp          # Start MCP server (blocks, communicates over stdin/stdout)
```

### Client Configuration

Add to Claude Desktop `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "niyantra": {
      "command": "C:\\path\\to\\niyantra.exe",
      "args": ["mcp"]
    }
  }
}
```

### Tools (9 total)

| Tool | Input | Description |
|------|-------|-------------|
| `quota_status` | none | All accounts with per-group readiness, remaining %, reset timers, **estimated cost** (F8) |
| `model_availability` | `model` (string) | Check specific model by name/keyword (fuzzy match) |
| `usage_intelligence` | none | Per-model rates, projections, exhaustion, cycle history |
| `budget_forecast` | none | Monthly burn rate, projected spend, on-track status |
| `best_model` | `group` (string) | Recommend least-exhausted model in a quota group |
| `analyze_spending` | none | Category breakdown, budget status, savings detection, insights |
| `switch_recommendation` | none | Account switch advice (stay/switch/wait) with scores |
| `codex_status` | none | Codex CLI detection, plan, token expiry, latest snapshot |
| `quota_forecast` | none | TTX forecasts with **per-group estimated cost** and $/hr (Phase 14: F7+F8) |

### Protocol

- **Transport:** stdio (newline-delimited JSON-RPC 2.0)
- **SDK:** `github.com/modelcontextprotocol/go-sdk` v1.5.0
- **Protocol version:** `2025-03-26`
- **Server info:** `name: "niyantra"`, `version: "1.0.0"`

---

## Phase 9 Endpoints

### `GET /api/claude/status`

Returns Claude Code rate limit data from the statusline bridge.

**Response:** `200 OK`

```json
{
  "installed": true,
  "bridgeEnabled": true,
  "bridgeFresh": true,
  "supported": true,
  "snapshot": {
    "fiveHourPct": 42.5,
    "sevenDayPct": 15.0,
    "fiveHourReset": "2026-04-18T18:00:00Z",
    "sevenDayReset": "2026-04-24T00:00:00Z",
    "capturedAt": "2026-04-18T17:15:00Z",
    "source": "statusline"
  }
}
```

---

### `GET /api/backup`

Downloads the database file as an attachment.

**Response:** `200 OK` with `Content-Type: application/octet-stream`

**Headers:**
- `Content-Disposition: attachment; filename="niyantra-2026-04-18.db"`

---

### `POST /api/notify/test`

Sends a test OS-native desktop notification.

**Response:** `200 OK`

```json
{ "status": "sent" }
```

**Error:** `400 Bad Request` if notifications not supported on the platform.

---

## Phase 10 Endpoints

### `GET /api/export/json`

Full JSON export of all data (accounts, subscriptions, snapshots, claude data, config).

**Response:** `200 OK` — JSON file download

```json
{
  "exportedAt": "2026-04-18T21:00:00Z",
  "version": "niyantra-export-v1",
  "accounts": [...],
  "subscriptions": [...],
  "snapshots": [...],
  "claudeSnapshots": [...],
  "activityLog": [...],
  "config": [...]
}
```

---

### `GET /api/alerts`

Returns active (non-dismissed) system alerts, ordered by severity.

**Response:** `200 OK`

```json
{
  "alerts": [
    {
      "id": 1,
      "severity": "critical",
      "category": "quota",
      "message": "All Claude+GPT quota exhausted on 3 accounts",
      "createdAt": "2026-04-18T21:00:00Z",
      "expiresAt": null
    }
  ]
}
```

---

### `POST /api/alerts/dismiss`

Dismiss an alert by ID.

**Request Body:**

```json
{ "id": 1 }
```

**Response:** `200 OK` `{ "status": "ok" }`

---

### `GET /api/advisor`

Returns the switch advisor recommendation based on current account health.

**Response:** `200 OK`

```json
{
  "action": "stay",
  "reason": "Best account is user@gmail.com with 87% remaining (score 72). No significant advantage in switching.",
  "bestAccount": {
    "email": "user@gmail.com",
    "score": 72,
    "remainingPct": 87,
    "burnRate": 0,
    "minutesToReset": 280
  },
  "alternatives": [
    {
      "email": "other@gmail.com",
      "score": 64,
      "remainingPct": 60,
      "burnRate": 0.5,
      "minutesToReset": 180
    }
  ]
}
```

**Actions:**
- `stay` — Current account is best or comparable
- `switch` — Another account has significantly better score (≥15 point gap)
- `wait` — All accounts exhausted; shows shortest reset time

---

## Phase 11: Codex & Sessions

### `GET /api/codex/status`

Returns Codex CLI detection state, account info, token expiry, and latest snapshot.

```json
{
  "installed": true,
  "captureEnabled": false,
  "accountId": "1dd7f5aa-c097-44b4-a70f-3b8cd6ee128e",
  "tokenExpiry": "2026-04-20T01:37:00Z",
  "tokenExpiresIn": "12h0m",
  "tokenExpired": false,
  "snapshot": { "fiveHourPct": 23.5, "planType": "plus", ... }
}
```

---

### `POST /api/codex/snap`

Triggers a manual Codex usage snapshot. Auto-refreshes expired tokens.

**Response:** `200 OK`

```json
{
  "message": "Codex snapshot captured",
  "snapshotId": 42,
  "plan": "plus",
  "quotas": [{ "name": "five_hour", "utilization": 23.5 }]
}
```

---

### `GET /api/sessions`

Returns recent usage sessions. Optional `?provider=codex|antigravity|claude&limit=50`.

```json
{
  "sessions": [
    { "id": 1, "provider": "antigravity", "startedAt": "...", "endedAt": "...", "durationSec": 1800, "snapCount": 6 }
  ],
  "count": 1
}
```

---

### `GET /api/usage-logs?subscriptionId=1`

Returns usage logs for a subscription. Required: `subscriptionId`. Optional: `limit`.

```json
{
  "logs": [{ "id": 1, "subscriptionId": 1, "usageAmount": 50, "usageUnit": "requests", "notes": "..." }],
  "summary": { "totalAmount": 150, "logCount": 3, "lastUnit": "requests" }
}
```

---

### `POST /api/usage-logs`

Creates a manual usage log entry.

**Body:**
```json
{ "subscriptionId": 1, "usageAmount": 50, "usageUnit": "requests", "notes": "Daily API usage" }
```

---

### `DELETE /api/usage-logs/{id}`

Deletes a usage log entry by ID.

---

### `POST /api/import/json`

Imports data from a Niyantra JSON export with additive merge strategy.

**Body:** Raw JSON export file content.

**Response:** `200 OK`

```json
{
  "accountsCreated": 1,
  "accountsSkipped": 0,
  "subsCreated": 3,
  "subsSkipped": 1,
  "snapshotsImported": 45,
  "snapshotsDuped": 2,
  "errors": []
}
```

---

---

## Phase 13 Endpoints

### `PATCH /api/accounts/:id/meta`

Updates account notes, tags, pinned group, and/or credit renewal day. Supports partial updates — omitted fields are preserved.

**Request:** `application/json`

```json
{
  "notes": "Main work account",
  "tags": "work,primary",
  "creditRenewalDay": 7
}
```

| Field | Type | Description |
|-------|------|-------------|
| `notes` | `string?` | Free-text note (max 100 chars). Omit to preserve current. |
| `tags` | `string?` | Comma-separated tags (alphanumeric + underscore/dash). Omit to preserve. |
| `pinnedGroup` | `string?` | Pinned quota group key (`claude_gpt`, `gemini_pro`, etc.). Omit to preserve. |
| `creditRenewalDay` | `int?` | Day of month (1-31) when AI credits refresh. 0 to clear. Omit to preserve. |

**Response (success):** `200 OK`

```json
{
  "message": "account meta updated",
  "notes": "Main work account",
  "tags": "work,primary",
  "pinnedGroup": "",
  "creditRenewalDay": 7
}
```

**Response (not found):** `404 Not Found`

```json
{ "error": "account not found" }
```

---

### MCP Tool: `codex_status`

Returns Codex detection state, token info, and latest snapshot for AI agents.

```json
{
  "installed": true,
  "captureEnabled": false,
  "accountId": "...",
  "tokenExpired": false,
  "snapshot": { "fiveHourPct": 23.5, "planType": "plus" },
  "message": "Codex active (account ...). 5h: 23.5% used, plan: plus."
}
```

---

## Phase 13: Model Pricing Config (F5)

### `GET /api/config/pricing`

Returns per-model token pricing. On first call, seeds with current market defaults (Claude Opus/Sonnet/Haiku, GPT-4o, Gemini Pro/Flash).

**Response:** `200 OK`

```json
{
  "pricing": [
    {
      "modelId": "claude-sonnet-4.6",
      "displayName": "Claude Sonnet 4.6",
      "provider": "anthropic",
      "inputPer1M": 3.00,
      "outputPer1M": 15.00,
      "cachePer1M": 0.30
    },
    {
      "modelId": "gpt-4o",
      "displayName": "GPT-4o",
      "provider": "openai",
      "inputPer1M": 2.50,
      "outputPer1M": 10.00,
      "cachePer1M": 1.25
    }
  ]
}
```

**Field Reference — ModelPrice:**

| Field | Type | Description |
|-------|------|-------------|
| `modelId` | `string` | Unique model identifier (e.g., `claude-sonnet-4.6`) |
| `displayName` | `string` | Human-readable name |
| `provider` | `string` | Provider key: `anthropic`, `openai`, `google`, or `custom` |
| `inputPer1M` | `float64` | Cost per 1M input tokens ($) |
| `outputPer1M` | `float64` | Cost per 1M output tokens ($) |
| `cachePer1M` | `float64` | Cost per 1M cached/prompt-cache tokens ($) |

---

### `PUT /api/config/pricing`

Updates the full pricing configuration. Replaces all existing entries.

**Request:** `application/json`

```json
{
  "pricing": [
    {
      "modelId": "claude-sonnet-4.6",
      "displayName": "Claude Sonnet 4.6",
      "provider": "anthropic",
      "inputPer1M": 4.00,
      "outputPer1M": 20.00,
      "cachePer1M": 0.40
    }
  ]
}
```

**Validation:**
- `pricing` array must not be empty
- Each entry requires a non-empty `modelId`
- All price values must be ≥ 0

**Response (success):** `200 OK`

```json
{
  "message": "pricing updated",
  "pricing": [...]
}
```

**Response (validation error):** `400 Bad Request`

```json
{ "error": "each pricing entry requires a modelId" }
```

---

**Storage:** Model pricing is stored as a JSON blob in the `config` table under the key `model_pricing`. No schema migration is required — it uses the existing config key-value infrastructure with `INSERT OR IGNORE ... ON CONFLICT` upsert.

**Prerequisite for:** F8 (Estimated Cost Tracking) — pricing data will be used to compute cost from quota deltas.
