# API Specification: Niyantra

## Base URL

```
http://localhost:9222
```

## Authentication

Optional. If `--auth user:pass` is provided at startup, all endpoints require HTTP Basic Auth.

---

## Endpoints

### `GET /healthz`

Liveness/health check endpoint for monitoring and container orchestration. **No authentication required.**

**Response:** `200 OK`

```json
{
  "status": "ok",
  "version": "0.12.0",
  "uptime": "2h15m30s",
  "schemaVersion": 11,
  "accounts": 2,
  "snapshots": 47
}
```

---

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

### `GET /api/history/heatmap` (Phase 14: F6)

Returns daily snapshot counts across all providers (Antigravity, Claude Code, Codex, Cursor, Gemini CLI) for rendering a GitHub-style contribution calendar.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `days` | `int` | 365 | Number of days of history (max 730) |

**Response:** `200 OK`

```json
{
  "days": [
    { "date": "2026-05-12", "count": 6, "antigravity": 3, "claude": 1, "codex": 1, "cursor": 0, "gemini": 1 },
    { "date": "2026-05-11", "count": 14, "antigravity": 8, "claude": 2, "codex": 2, "cursor": 1, "gemini": 1 }
  ],
  "maxCount": 12,
  "totalSnapshots": 847,
  "activeDays": 45,
  "streak": 7,
  "longestStreak": 14
}
```

**Field Reference:**

| Field | Type | Description |
|-------|------|-------------|
| `days` | array | Per-day snapshot counts, ordered chronologically ASC |
| `days[].date` | string | Date in YYYY-MM-DD format |
| `days[].count` | int | Total snapshots across all providers |
| `days[].antigravity` | int | Antigravity snapshot count |
| `days[].claude` | int | Claude Code snapshot count |
| `days[].codex` | int | Codex snapshot count |
| `days[].cursor` | int | Cursor snapshot count |
| `days[].gemini` | int | Gemini CLI snapshot count |
| `maxCount` | int | Highest single-day count (used for intensity scaling) |
| `totalSnapshots` | int | Sum of all snapshot counts in the range |
| `activeDays` | int | Number of days with at least 1 snapshot |
| `streak` | int | Current consecutive-day activity streak |
| `longestStreak` | int | Longest consecutive-day activity streak ever |

> **Data Source:** UNION ALL query across `snapshots`, `claude_snapshots`, `codex_snapshots`, `cursor_snapshots`, and `gemini_snapshots` tables, grouped by `date(captured_at)`.

---

### `GET /api/claude/usage` (Phase 14: F15d)

Returns deep token usage analytics from Claude Code's local JSONL session files (`~/.claude/projects/*/sessions/*.jsonl`). Zero network calls — pure filesystem parsing.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `days` | `int` | 30 | Number of days of history (max 365) |

**Response:** `200 OK`

```json
{
  "days": [
    {
      "date": "2026-05-12",
      "totalInput": 45000,
      "totalOutput": 12000,
      "totalCacheRead": 30000,
      "totalCacheCreate": 5000,
      "totalCost": 0.42,
      "sessionCount": 3,
      "byModel": {
        "claude-sonnet-4": {
          "model": "claude-sonnet-4",
          "inputTokens": 30000,
          "outputTokens": 8000,
          "cacheRead": 20000,
          "cacheCreate": 3000,
          "costUSD": 0.28,
          "turns": 15
        }
      }
    }
  ],
  "totalCost": 12.50,
  "totalInput": 800000,
  "totalOutput": 450000,
  "totalTokens": 1250000,
  "totalSessions": 47,
  "cacheHitRate": 0.65,
  "topModel": "claude-sonnet-4"
}
```

**Field Reference:**

| Field | Type | Description |
|-------|------|-------------|
| `days` | array | Per-day token/cost aggregations, ordered ASC |
| `totalCost` | float | Estimated cost in USD (using F5 model pricing) |
| `totalTokens` | int | Total input + output tokens |
| `totalSessions` | int | Unique Claude Code sessions in the range |
| `cacheHitRate` | float | Cache read / (cache read + cache create), 0.0-1.0 |
| `topModel` | string | Most-used model by total token count |

> **Data Source:** Parses `~/.claude/projects/*/sessions/*.jsonl` files. Extracts `message.usage` from `type: "assistant"` records. Cost estimated via `store.GetModelPrice()` (F5 pricing config).

---

### `GET /api/cursor/status` (Phase 14: F15a)

Returns Cursor IDE detection state, credential info, and latest usage snapshot.

**Response:** `200 OK`

```json
{
  "installed": true,
  "captureEnabled": false,
  "email": "user@example.com",
  "userId": "user_abc123def456ghi789",
  "source": "auto",
  "snapshot": {
    "billingModel": "usd_credit",
    "planTier": "pro",
    "requestsUsed": 0,
    "requestsMax": 0,
    "usedCents": 1250,
    "limitCents": 50000,
    "usagePct": 2.5,
    "autoPct": 1.8,
    "apiPct": 0.7,
    "cycleStart": "1715558400000",
    "cycleEnd": "1718236800000",
    "capturedAt": "2026-05-13T08:00:00Z"
  }
}
```

**Field Reference:**

| Field | Type | Description |
|-------|------|-------------|
| `installed` | bool | Whether Cursor credentials were detected locally |
| `captureEnabled` | bool | Whether auto-polling is enabled (`cursor_capture` config) |
| `email` | string | Email from Cursor's cached auth (may be empty) |
| `userId` | string | `user_xxx` ID from sentry/scope_v3.json |
| `source` | string | `"auto"` (detected from state.vscdb) or `"manual"` (config token) |
| `snapshot.billingModel` | string | `"request_count"` (legacy) or `"usd_credit"` (new) |
| `snapshot.planTier` | string | `free`, `pro`, `pro_plus`, `ultra`, `team`, or `unknown` |
| `snapshot.usedCents` | int | USD cents consumed this cycle (credit billing only) |
| `snapshot.limitCents` | int | USD cents limit per cycle (credit billing only) |
| `snapshot.requestsUsed` | int | Requests used (legacy billing only) |
| `snapshot.requestsMax` | int | Max requests per cycle (legacy billing only) |
| `snapshot.usagePct` | float | Overall usage percentage (0-100) |

> **Credential Detection:** Auto-reads `accessToken` from `state.vscdb` (SQLite) + `userId` from `sentry/scope_v3.json`. Falls back to `cursor_session_token` config key. Three endpoints polled: `cursor.com/api/usage` (Cookie), `api2.cursor.sh/GetCurrentPeriodUsage` (Bearer), `cursor.com/api/auth/stripe` (Cookie).

---

### `POST /api/cursor/snap` (Phase 14: F15a)

Triggers a manual Cursor usage snapshot.

**Response:** `200 OK`

```json
{
  "message": "Cursor snapshot captured",
  "snapshotId": 7,
  "billingModel": "usd_credit",
  "planTier": "pro",
  "usagePct": 2.5,
  "requestsUsed": 0,
  "requestsMax": 0,
  "usedCents": 1250,
  "limitCents": 50000
}
```

**Error Responses:**
- `400` — Cursor not detected (no token or userId found)
- `502` — Cursor API error (all 3 endpoints failed, or unauthorized)

---

### `GET /api/gemini/status` (Phase 14: F15b)

Returns Gemini CLI detection state, credential info, and latest usage snapshot.

**Response:** `200 OK`

```json
{
  "installed": true,
  "captureEnabled": false,
  "email": "user@gmail.com",
  "source": "auto",
  "expired": false,
  "hasRefreshToken": true,
  "snapshot": {
    "id": 5,
    "accountId": 12,
    "email": "user@gmail.com",
    "tier": "standard",
    "overallPct": 25.0,
    "modelsJson": "[{\"modelId\":\"gemini-2.5-flash\",\"remainingFraction\":0.75,\"usedPct\":25.0,\"resetTime\":\"2026-05-14T00:00:00Z\",\"tier\":\"flash\"}]",
    "projectId": "project-123",
    "capturedAt": "2026-05-13T08:00:00Z",
    "captureMethod": "auto",
    "captureSource": "server"
  }
}
```

**Field Reference:**

| Field | Type | Description |
|-------|------|-------------|
| `installed` | bool | Whether `~/.gemini/oauth_creds.json` was found |
| `captureEnabled` | bool | Whether auto-polling is enabled (`gemini_capture` config) |
| `email` | string | Email from Google userinfo (may be empty) |
| `source` | string | `"auto"` (detected from oauth_creds.json) |
| `expired` | bool | Whether the access token has expired |
| `hasRefreshToken` | bool | Whether a refresh token is available for auto-renewal |
| `snapshot.tier` | string | Google tier: `standard`, `enterprise`, `unknown` |
| `snapshot.overallPct` | float | Weighted average usage across all model buckets (0-100) |
| `snapshot.modelsJson` | string | JSON array of per-model quota buckets |
| `snapshot.projectId` | string | `cloudaicompanionProject` from loadCodeAssist |

> **Credential Detection:** Auto-reads `access_token`, `refresh_token`, `expiry_date` from `~/.gemini/oauth_creds.json`. Token auto-refresh via `https://oauth2.googleapis.com/token` using CLIENT_ID/SECRET from Gemini CLI npm package or `gemini_client_id`/`gemini_client_secret` config keys.

> **API Flow:** Two endpoints polled sequentially:
> 1. `POST cloudcode-pa.googleapis.com/v1internal:loadCodeAssist` → tier + project ID
> 2. `POST cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota` → per-model `{modelId, remainingFraction, resetTime}` buckets

---

### `POST /api/gemini/snap` (Phase 14: F15b)

Triggers a manual Gemini CLI usage snapshot.

**Response:** `200 OK`

```json
{
  "message": "Gemini CLI snapshot captured",
  "snapshotId": 5,
  "tier": "standard",
  "overallUsedPct": 25.0,
  "modelCount": 3
}
```

**Error Responses:**
- `400` — Gemini CLI not detected (`~/.gemini/oauth_creds.json` not found)
- `502` — Gemini API error (token expired, unauthorized, or network failure)

---

### `GET /api/copilot/status` (Phase 15: F15c)

Returns GitHub Copilot detection state and latest usage snapshot.

**Response:** `200 OK`

```json
{
  "configured": true,
  "captureEnabled": false,
  "snapshot": {
    "id": 3,
    "accountId": 15,
    "email": "user@github.com",
    "username": "octocat",
    "plan": "Pro",
    "premiumPct": 35.5,
    "chatPct": 12.0,
    "hasPremium": true,
    "hasChat": true,
    "capturedAt": "2026-05-16T12:00:00Z",
    "captureMethod": "manual",
    "captureSource": "ui"
  }
}
```

**Field Reference:**

| Field | Type | Description |
|-------|------|-------------|
| `configured` | bool | Whether a `copilot_pat` config key is set |
| `captureEnabled` | bool | Whether auto-polling is enabled (`copilot_capture` config) |
| `snapshot.plan` | string | `Pro`, `Pro+`, `Free`, `Business`, `Enterprise`, `unknown` |
| `snapshot.premiumPct` | float | Premium interactions usage % (0-100) |
| `snapshot.chatPct` | float | Chat usage % (0-100) |
| `snapshot.hasPremium` | bool | Whether premium data is available |
| `snapshot.hasChat` | bool | Whether chat data is available |
| `snapshot.username` | string | GitHub username |

> **Authentication:** User provides a GitHub PAT with `read:user` scope via Settings → `copilot_pat` config key. The PAT is used as `Authorization: token <PAT>` to query the Copilot internal usage API.

> **API Flow:** Two endpoints polled:
> 1. `GET api.github.com/copilot_internal/user` → quota snapshots (premiumInteractions, chat) + plan
> 2. `GET api.github.com/user` → username/email (best-effort)

---

### `POST /api/copilot/snap` (Phase 15: F15c)

Triggers a manual GitHub Copilot usage snapshot.

**Response:** `200 OK`

```json
{
  "message": "Copilot snapshot captured",
  "snapshotId": 3,
  "plan": "Pro",
  "premiumPct": 35.5,
  "chatPct": 12.0,
  "username": "octocat"
}
```

**Error Responses:**
- `400` — Copilot PAT not configured
- `502` — Copilot API error (PAT invalid, Copilot not enabled, or network failure)

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

---

### `GET /api/token-usage` (Phase 15: F13)

Returns unified token usage analytics aggregating Claude Code JSONL sessions (full per-turn granularity) with estimated data from snapshot providers.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `days` | `int` | 30 | Number of days to analyze (max 365) |
| `provider` | `string` | `all` | Filter: `all`, `claude`, `antigravity`, `codex`, `cursor`, `gemini` |

**Response:** `200 OK`

```json
{
  "totals": {
    "totalTokens": 1250000,
    "inputTokens": 800000,
    "outputTokens": 450000,
    "cacheTokens": 350000,
    "estCostUSD": 12.50,
    "sessions": 47
  },
  "kpis": {
    "daysActive": 14,
    "avgTokensPerDay": 89285,
    "cacheHitRate": 0.65,
    "topModel": "claude-sonnet-4",
    "peakDay": "2026-05-10"
  },
  "byModel": [
    { "model": "claude-sonnet-4", "tokens": 900000, "costUSD": 8.50, "pct": 72 }
  ],
  "daily": [
    { "date": "2026-05-12", "tokens": 120000, "costUSD": 1.20 }
  ],
  "period": { "start": "2026-04-14", "end": "2026-05-14", "days": 30 }
}
```

> **Data Source:** Primary: Claude Code JSONL (`~/.claude/projects/*/sessions/*.jsonl`). Secondary: estimated data from `token_usage` table (schema v14) for all other providers.

---

### `GET /api/git-costs` (Phase 15: F16)

Correlates git commits with actual AI token consumption from Claude Code sessions. For each commit, finds overlapping Claude sessions within a ±30 min time window and reports real per-commit cost.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `repo` | `string` | CWD | Path to git repository |
| `days` | `int` | 30 | Number of days to analyze (max 365) |

**Response:** `200 OK`

```json
{
  "commits": [
    {
      "hash": "abc1234def5678...",
      "shortHash": "abc1234",
      "date": "2026-05-14T10:30:00+05:30",
      "dateStr": "2026-05-14",
      "message": "feat: add authentication",
      "author": "user",
      "branch": "main",
      "inputTokens": 30000,
      "outputTokens": 15000,
      "cacheTokens": 20000,
      "totalTokens": 45000,
      "costUSD": 0.85,
      "sessions": 2,
      "turns": 12
    }
  ],
  "branches": [
    { "name": "feat/token-usage", "commits": 3, "totalTokens": 150000, "costUSD": 3.50, "avgPerCommit": 1.17 }
  ],
  "totals": {
    "commitCount": 42,
    "totalTokens": 500000,
    "costUSD": 12.34,
    "avgPerCommit": 0.29,
    "topBranch": "feat/token-usage"
  },
  "period": { "start": "2026-04-14", "end": "2026-05-14", "days": 30 },
  "repoPath": "D:\\dev\\pro\\niyantra"
}
```

**Field Reference — Commit Cost:**

| Field | Type | Description |
|-------|------|-------------|
| `hash` | string | Full commit SHA |
| `shortHash` | string | 7-char abbreviated SHA |
| `totalTokens` | int | Input + output tokens consumed during this commit's time window |
| `costUSD` | float | Estimated cost from F5 model pricing |
| `sessions` | int | Number of distinct Claude Code sessions correlated |
| `turns` | int | Number of AI assistant turns in the time window |

> **Algorithm:** Runs `git log --all --no-merges --format` to extract commits, then for each commit timestamp finds Claude Code JSONL session records in `[commit_time - 30min, commit_time]`. Cost computed via `store.GetModelPrice()` with fuzzy prefix matching. No database writes — pure computation.

> **Unique Feature:** No competitor does cost correlation with actual token data. `semcod/costs` estimates from diff size; Niyantra uses real Claude Code session telemetry.

---

### `POST /api/notify/test` (Phase 9)

Sends a test OS-native desktop notification to verify the platform notification system works.

**Response (success):** `200 OK`

```json
{ "status": "sent" }
```

**Response (unsupported):** `400 Bad Request`

```json
{ "error": "notifications not supported on this platform" }
```

---

### `POST /api/notify/test-email` (Phase 16: F11)

Sends a test email to verify SMTP configuration. Requires `smtp_enabled=true` and all SMTP config keys to be set.

**Response (success):** `200 OK`

```json
{ "status": "sent" }
```

**Response (not configured):** `500 Internal Server Error`

```json
{ "error": "email failed: SMTP is not configured" }
```

**SMTP Config Keys (stored in `config` table):**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `smtp_enabled` | bool | `false` | Master toggle for SMTP email delivery |
| `smtp_host` | string | `""` | SMTP server hostname (e.g. `smtp.gmail.com`) |
| `smtp_port` | int | `587` | SMTP port: 587 (STARTTLS), 465 (TLS), 25 (plain) |
| `smtp_user` | string | `""` | SMTP authentication username |
| `smtp_pass` | string | `""` | SMTP password (masked as `"configured"` in GET response) |
| `smtp_from` | string | `""` | Sender email address |
| `smtp_to` | string | `""` | Recipient email address(es), comma-separated |
| `smtp_tls` | string | `"starttls"` | Encryption mode: `starttls`, `tls`, `none` |

> **Quad-Channel Delivery:** When quota alerts fire (F9), notifications are sent via OS-native desktop, SMTP email (if configured), Webhook (if configured), and WebPush (if configured). All async channels fire in independent goroutines.

---

### `POST /api/notify/test-webhook` (Phase 16: F22)

Sends a test notification to verify webhook configuration. Supports Discord, Telegram, Slack, and generic (ntfy/Gotify) endpoints.

**Response (success):** `200 OK`

```json
{ "status": "sent" }
```

**Response (not configured):** `500 Internal Server Error`

```json
{ "error": "webhook failed: webhook is not configured" }
```

**Webhook Config Keys (stored in `config` table):**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `webhook_enabled` | bool | `false` | Master toggle for webhook delivery |
| `webhook_type` | string | `"discord"` | Service type: `discord`, `telegram`, `slack`, `generic` |
| `webhook_url` | string | `""` | Webhook URL (Discord/Slack), Chat ID (Telegram), or endpoint URL (Generic) |
| `webhook_secret` | string | `""` | Bot token (Telegram), auth header (Generic); masked in GET response |

**Supported Services:**

| Service | URL Field | Secret Field | Payload Format |
|---------|-----------|-------------|---------------|
| Discord | Webhook URL | _(unused)_ | JSON embed with severity color |
| Telegram | Chat ID (numeric) | Bot token from @BotFather | HTML-formatted sendMessage |
| Slack | Incoming webhook URL | _(unused)_ | JSON attachment with color |
| Generic (ntfy) | POST endpoint URL | Auth header (optional) | Plain text body + Title/Priority headers |

---

### WebPush Notifications (Phase 16: F19)

Browser push notifications using VAPID (RFC 8292) + RFC 8291 payload encryption. Works on Chrome, Firefox, Edge, Safari 16+. Zero external Go dependencies — HKDF implemented via stdlib `crypto/hmac`.

#### `GET /api/webpush/vapid-key`

Returns the public VAPID key (auto-generates P-256 key pair on first request).

```json
{ "publicKey": "BNxBr..." }
```

#### `POST /api/webpush/subscribe`

Stores a browser push subscription. Body is the standard `PushSubscription.toJSON()` output:

```json
{
  "endpoint": "https://fcm.googleapis.com/fcm/send/...",
  "keys": { "auth": "...", "p256dh": "..." }
}
```

**Response:** `200 OK`
```json
{ "status": "subscribed" }
```

#### `DELETE /api/webpush/subscribe`

Removes a stored subscription by endpoint.

```json
{ "endpoint": "https://fcm.googleapis.com/fcm/send/..." }
```

#### `POST /api/notify/test-webpush`

Sends a test push notification to all stored subscriptions.

**Response:** `200 OK` / `500` with `{"error": "..."}`

#### `GET /api/webpush/status`

Returns WebPush status.

```json
{ "enabled": true, "subscriptions": 2, "has_vapid": true }
```

**WebPush Config Keys (stored in `config` table):**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `webpush_enabled` | bool | `false` | Master toggle for WebPush delivery |
| `webpush_vapid_public` | string | `""` | Auto-generated VAPID public key |
| `webpush_vapid_private` | string | `""` | Auto-generated VAPID private key (masked in GET) |

> **Quad-Channel Delivery:** When quota alerts fire (F9), notifications are sent via OS-native desktop, SMTP email (if configured), Webhook (if configured), and WebPush (if configured). All async channels fire in independent goroutines.

---

### Plugin Endpoints (Phase 16: F18)

Manage and query external data source plugins. Plugins are language-agnostic scripts in `~/.niyantra/plugins/` that extend Niyantra's tracking capabilities.

#### `GET /api/plugins`

Lists all discovered plugins with their manifests, enabled state, configuration, and capture stats.

**Response:** `200 OK`

```json
{
  "plugins": [
    {
      "manifest": {
        "id": "openrouter-usage",
        "name": "OpenRouter Usage Tracker",
        "version": "1.0.0",
        "description": "Tracks OpenRouter API credit usage",
        "author": "Niyantra Examples",
        "entryPoint": "capture.py",
        "timeout": 15,
        "config": {
          "api_key": { "type": "string", "label": "OpenRouter API Key", "required": true, "secret": true },
          "base_url": { "type": "string", "label": "API Base URL", "default": "https://openrouter.ai/api/v1" }
        }
      },
      "dir": "/home/user/.niyantra/plugins/openrouter-usage",
      "enabled": true,
      "config": { "base_url": "https://openrouter.ai/api/v1", "api_key": "••••••••" },
      "lastCapture": "2026-05-16T14:30:00Z",
      "captureCount": 12
    }
  ],
  "pluginsDir": "/home/user/.niyantra/plugins",
  "errors": []
}
```

| Field | Type | Description |
|-------|------|-------------|
| `plugins` | array | All discovered plugins with valid manifests |
| `plugins[].manifest` | object | Parsed `plugin.json` manifest |
| `plugins[].dir` | string | Absolute path to the plugin directory |
| `plugins[].enabled` | bool | Whether auto-polling captures this plugin |
| `plugins[].config` | object | Current config values (secrets masked) |
| `plugins[].lastCapture` | string? | ISO timestamp of last successful capture |
| `plugins[].captureCount` | int | Total number of captures stored |
| `pluginsDir` | string | Base plugins directory path |
| `errors` | array | Discovery errors (invalid manifests, permission issues) |

#### `GET /api/plugins/{id}/status`

Returns the latest snapshot data for a specific plugin.

**Response:** `200 OK`

```json
{
  "pluginId": "openrouter-usage",
  "provider": "openrouter",
  "label": "OpenRouter",
  "usagePct": 42.5,
  "usageDisplay": "$4.25 / $10.00",
  "plan": "api",
  "capturedAt": "2026-05-16T14:30:00Z",
  "captureCount": 12
}
```

**Response (no data):** `404 Not Found` — `{ "error": "no snapshot found" }`

#### `POST /api/plugins/{id}/run`

Triggers a manual test execution of a plugin. The plugin's subprocess is invoked immediately with its current config, and the captured data is returned (but not persisted).

**Response:** `200 OK`

```json
{
  "status": "ok",
  "data": {
    "provider": "openrouter",
    "label": "OpenRouter",
    "usage_pct": 42.5,
    "usage_display": "$4.25 / $10.00",
    "plan": "api"
  }
}
```

**Response (plugin error):** `200 OK` — `{ "status": "error", "error": "API key invalid" }`

**Response (plugin not found):** `404 Not Found` — `{ "error": "plugin 'xyz' not found" }`

#### `PUT /api/plugins/{id}/config`

Updates configuration for a plugin. Supports setting arbitrary key-value pairs (matched against the plugin's config schema) and the special `enabled` key for toggling auto-capture.

**Request:** `application/json`

```json
{
  "enabled": "true",
  "api_key": "sk-or-v1-abc123...",
  "base_url": "https://openrouter.ai/api/v1"
}
```

**Response:** `200 OK` — `{ "message": "config updated" }`

> **Config Storage:** Plugin config values are stored in Niyantra's existing `config` table using the key format `plugin_{id}_{key}` (e.g., `plugin_openrouter-usage_api_key`). Secret fields are masked in GET responses.

> **Plugin Protocol:** Plugins receive `{"action": "capture", "config": {...}}` on stdin and must return `{"status": "ok", "data": {...}}` on stdout. See `examples/plugins/openrouter-usage/` for a reference implementation.

---

### `POST /mcp` — Streamable HTTP MCP (Phase 15: F14)

Exposes all 12 MCP tools over HTTP using the MCP Streamable HTTP transport protocol. This enables remote MCP clients (Claude Desktop on another machine, CI/CD pipelines, cross-machine AI agents) to connect without stdio.

**No authentication required** — the MCP SDK handles its own transport-level security (Origin/Host header verification). The endpoint does not go through Niyantra's basic auth middleware.

**Protocol:** MCP JSON-RPC 2.0 over HTTP, with optional SSE streaming for server-to-client notifications.

**Supported methods:**
- `POST /mcp` — Send JSON-RPC requests (initialize, tools/list, tools/call)
- `GET /mcp` — Open SSE stream for server notifications (requires active session)
- `DELETE /mcp` — Terminate a session

**Example: List available tools**

```bash
# Initialize session
curl -X POST http://localhost:9222/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"curl","version":"1.0"}}}'

# List tools (use session ID from response)
curl -X POST http://localhost:9222/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id>" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
```

**Available tools (12):**

| Tool | Description |
|------|-------------|
| `quota_status` | All tracked accounts' quota status with readiness |
| `model_availability` | Check a specific model's remaining quota |
| `usage_intelligence` | Consumption rates and projections for all models |
| `budget_forecast` | Monthly budget burn rate and projections |
| `best_model` | Recommend optimal model by remaining quota |
| `analyze_spending` | Subscription spending patterns and insights |
| `switch_recommendation` | Which account to use right now |
| `codex_status` | Codex/ChatGPT detection and usage state |
| `quota_forecast` | Time-to-exhaustion predictions with severity |
| `token_usage_stats` | Unified token analytics across all providers |
| `git_commit_costs` | Git commit ↔ AI token cost correlation |
| `plugin_status` | Latest data from all installed external plugins |

> **Transport Note:** The same 12 tools are available via both stdio (`niyantra mcp`) and HTTP (`/mcp` on the web dashboard). The HTTP transport uses the MCP Go SDK's `NewStreamableHTTPHandler` with session management and SSE support built in.

> **Claude Desktop Config:** To connect Claude Desktop to a remote Niyantra instance, configure the MCP server URL as `http://<host>:9222/mcp` using the Streamable HTTP transport type.

---

### `GET /api/anomalies`

Returns detected cost anomalies using Z-score statistical analysis. Analyzes subscription and account spending history to identify days where spend exceeds 2σ above the rolling 30-day average.

**Response:** `200 OK`

```json
{
  "anomalies": [
    {
      "date": "2026-05-15",
      "amount": 45.20,
      "average": 12.50,
      "zScore": 2.61,
      "multiplier": 3.62,
      "severity": "warning",
      "budgetProjection": 1356.00
    }
  ],
  "analyzed": true,
  "dataPoints": 30
}
```

**Fields:**
- `zScore`: Standard deviations above the mean (≥2.0 = warning, ≥3.0 = critical)
- `multiplier`: How many times the average this day's spend represents
- `severity`: `"warning"` (2-3σ) or `"critical"` (>3σ)
- `budgetProjection`: Estimated monthly spend if this rate continues

**Notes:**
- Requires at least 7 days of data to produce meaningful results
- Returns empty `anomalies` array if insufficient data or no anomalies detected
- Uses `internal/forecast/anomaly.go` Z-score engine (zero dependencies)
