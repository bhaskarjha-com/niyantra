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
| `email` | `string` | Antigravity account email |
| `planName` | `string` | Subscription plan (Free, Pro, Enterprise) |
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

## Client-Side Features (No API Required)

These features are implemented entirely in the browser using `localStorage`. No backend endpoints needed.

### Budget Threshold

- **Storage key:** `niyantra-budget`
- **Value:** monthly budget as float (e.g., `200`)
- **UI:** Budget alert bar on Overview tab (ok/warning ≥80%/danger ≥100%)
- **Configurable from:** Overview tab "Set Budget" button, Settings tab, or Budget modal

### Default Currency

- **Storage key:** `niyantra-currency`
- **Value:** ISO currency code (`USD`, `EUR`, `GBP`, `INR`, `CAD`, `AUD`)
- **UI:** Settings tab dropdown

### Theme Preference

- **Storage key:** `niyantra-theme`
- **Value:** `dark`, `light`, or absent (system default)
- **UI:** Settings tab dropdown or header toggle button

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
