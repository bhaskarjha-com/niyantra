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

Serves the single-page dashboard (embedded HTML).

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
      "staleness": 1260,
      "stalenessLabel": "21 min ago",
      "isReady": true,
      "groups": [
        {
          "groupKey": "claude_gpt",
          "displayName": "Claude + GPT",
          "remainingPercent": 40.0,
          "isExhausted": false,
          "isReady": true,
          "color": "#D97757",
          "resetTime": "2026-04-17T04:24:00Z",
          "timeUntilReset": 13800
        },
        {
          "groupKey": "gemini_pro",
          "displayName": "Gemini Pro",
          "remainingPercent": 100.0,
          "isExhausted": false,
          "isReady": true,
          "color": "#10B981",
          "resetTime": "2026-04-17T04:48:00Z",
          "timeUntilReset": 15240
        },
        {
          "groupKey": "gemini_flash",
          "displayName": "Gemini Flash",
          "remainingPercent": 100.0,
          "isExhausted": false,
          "isReady": true,
          "color": "#3B82F6",
          "resetTime": "2026-04-17T04:48:00Z",
          "timeUntilReset": 15240
        }
      ]
    },
    {
      "accountId": 2,
      "email": "personal@gmail.com",
      "planName": "Free",
      "lastSeen": "2026-04-16T23:15:00Z",
      "staleness": 5700,
      "stalenessLabel": "1h ago",
      "isReady": false,
      "groups": [
        {
          "groupKey": "claude_gpt",
          "displayName": "Claude + GPT",
          "remainingPercent": 0.0,
          "isExhausted": true,
          "isReady": false,
          "color": "#D97757",
          "resetTime": "2026-04-17T01:15:00Z",
          "timeUntilReset": 2700
        }
      ]
    }
  ],
  "snapshotCount": 147,
  "accountCount": 2
}
```

**Field Reference:**

| Field | Type | Description |
|-------|------|-------------|
| `staleness` | `float64` | Seconds since last snapshot |
| `stalenessLabel` | `string` | Human-readable staleness ("21 min ago", "3h ago") |
| `isReady` | `bool` | `true` if ALL groups have remaining > 0 |
| `timeUntilReset` | `float64` | Seconds until the group's quota resets |
| `remainingPercent` | `float64` | 0-100, average across models in the group |

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
  "snapshotId": 148,
  "accountId": 1,
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
      "id": 148,
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

## Error Format

All errors use a consistent JSON envelope:

```json
{
  "error": "human-readable error message"
}
```

HTTP status codes:
- `400` — Bad request (invalid parameters)
- `405` — Method not allowed
- `500` — Internal server error (database failure)
- `502` — Bad gateway (Antigravity language server unreachable)
- `503` — Service unavailable (database not initialized)

## CORS

Not needed. The dashboard is served from the same origin as the API.

## Rate Limiting

No rate limiting implemented. The tool is single-user by design.
