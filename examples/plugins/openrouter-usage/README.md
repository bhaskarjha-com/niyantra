# OpenRouter Usage Tracker Plugin

A reference Niyantra plugin that tracks [OpenRouter](https://openrouter.ai) API credit usage.

## Installation

```bash
# Copy to Niyantra plugins directory
cp -r examples/plugins/openrouter-usage ~/.niyantra/plugins/

# Or on Windows
xcopy examples\plugins\openrouter-usage %USERPROFILE%\.niyantra\plugins\openrouter-usage\ /E /I
```

## Configuration

1. Open Niyantra → Settings → Plugins
2. Enter your OpenRouter API key (from [keys page](https://openrouter.ai/settings/keys))
3. Toggle the plugin **ON**
4. Click **▶ Test Run** to verify

## How It Works

This plugin demonstrates the full Niyantra plugin protocol:

1. Niyantra invokes `capture.py` as a subprocess
2. Sends `{"action": "capture", "config": {"api_key": "...", "base_url": "..."}}` via **stdin**
3. Plugin calls `GET /api/v1/auth/key` on OpenRouter
4. Returns `{"status": "ok", "data": {...}}` via **stdout**

## Writing Your Own Plugin

Use this as a template! A plugin needs:

1. **`plugin.json`** — Manifest with id, name, entryPoint, and config schema
2. **Executable script** — Any language (Python, Node.js, Go, Bash, PowerShell, Ruby)
3. **JSON protocol** — Read from stdin, write to stdout

### Output Schema

```json
{
  "status": "ok",
  "data": {
    "provider": "your-service",
    "label": "Display Name",
    "usage_pct": 42.5,
    "usage_display": "425 / 1000 requests",
    "plan": "pro",
    "models": [],
    "metadata": { "custom_field": "value" }
  }
}
```

### Error Response

```json
{
  "status": "error",
  "error": "Human-readable error message"
}
```

## Requirements

- Python 3.6+ (uses only stdlib — no pip dependencies)
