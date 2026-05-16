#!/usr/bin/env python3
"""
OpenRouter Usage Tracker — Niyantra Plugin Example
===================================================

This is a reference plugin that demonstrates the Niyantra plugin protocol.
It queries the OpenRouter API for credit balance and usage.

Protocol:
  - Receives JSON on stdin:  {"action": "capture", "config": {...}}
  - Returns  JSON on stdout: {"status": "ok", "data": {...}}

Install:
  1. Copy this folder to ~/.niyantra/plugins/openrouter-usage/
  2. In Niyantra Settings → Plugins, enter your OpenRouter API key
  3. Enable the plugin toggle

Requirements: Python 3.6+ (uses only stdlib)
"""

import json
import sys
import urllib.request
import urllib.error


def capture(config: dict) -> dict:
    """Query OpenRouter /auth/key endpoint for credit balance."""
    api_key = config.get("api_key", "")
    if not api_key:
        return {"status": "error", "error": "OpenRouter API key is required"}

    base_url = config.get("base_url", "https://openrouter.ai/api/v1")
    url = f"{base_url}/auth/key"

    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Bearer {api_key}")
    req.add_header("Content-Type", "application/json")

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            body = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        return {"status": "error", "error": f"OpenRouter API error: {e.code}"}
    except Exception as e:
        return {"status": "error", "error": f"Network error: {str(e)}"}

    # Parse response: {"data": {"label": "...", "usage": 1.23, "limit": 50.0, ...}}
    data = body.get("data", {})
    usage = data.get("usage", 0.0)
    limit = data.get("limit", 0.0)
    label = data.get("label", "OpenRouter")

    # Calculate usage percentage
    if limit and limit > 0:
        usage_pct = (usage / limit) * 100
        usage_display = f"${usage:.2f} / ${limit:.2f}"
    elif usage > 0:
        usage_pct = 0  # No limit set — usage-based
        usage_display = f"${usage:.2f} used (no limit)"
    else:
        usage_pct = 0
        usage_display = "No usage"

    return {
        "status": "ok",
        "data": {
            "provider": "openrouter",
            "label": label or "OpenRouter",
            "usage_pct": round(usage_pct, 1),
            "usage_display": usage_display,
            "plan": "api",
            "models": [],
            "metadata": {
                "usage_usd": usage,
                "limit_usd": limit,
                "rate_limit_credits": data.get("rate_limit", {}).get("credits", 0),
            }
        }
    }


def main():
    """Read action from stdin, execute, write result to stdout."""
    try:
        raw = sys.stdin.read()
        request = json.loads(raw) if raw.strip() else {}
    except json.JSONDecodeError:
        json.dump({"status": "error", "error": "Invalid JSON input"}, sys.stdout)
        return

    action = request.get("action", "")
    config = request.get("config", {})

    if action == "capture":
        result = capture(config)
    else:
        result = {"status": "error", "error": f"Unknown action: {action}"}

    json.dump(result, sys.stdout)


if __name__ == "__main__":
    main()
