# ADR-0001: Plugin System Architecture

| Field        | Value                                      |
| ------------ | ------------------------------------------ |
| **Status**   | Accepted                                   |
| **Date**     | 2026-05-17                                 |
| **Authors**  | Bhaskar Jha                                |
| **Feature**  | F18 Plugin System (Phase 16)               |
| **Deciders** | Project maintainer                         |

## Context

Niyantra is a local-first, single-binary AI operations dashboard built in Go. It currently tracks 7 AI providers (Antigravity, Claude Code, Codex/ChatGPT, Cursor, Gemini CLI, GitHub Copilot) via hardcoded internal packages (`internal/{provider}/`). Each provider requires a Go package, store layer, web handlers, agent polling code, and MCP tool registration.

As the AI tool landscape expands rapidly, users want to track additional services (OpenRouter, Groq, Perplexity, Together AI, local LLM servers, etc.) without waiting for official Niyantra releases. The Plugin System (F18) enables users to write their own data collection scripts in any language and have Niyantra poll and store the results alongside built-in providers.

### Constraints

- **Single binary distribution** — no new runtime dependencies; binary size must stay reasonable
- **Pure Go, no CGo** — cross-compilation must remain trivial (`GOOS=X GOARCH=Y go build`)
- **Local-first** — all intelligence runs on user's machine
- **Windows + Linux + macOS** — must work on all three platforms
- **Minimal dependency footprint** — currently only 2 direct deps (MCP SDK + SQLite)
- **Target users** — developers who write Python, Bash, Node scripts (not Lua, Go, or Rust)
- **Use case** — periodic data capture (every 30s–5min), not real-time streaming

## Decision

**We will implement a Telegraf-inspired subprocess exec plugin system using Go's standard library (`os/exec` + `encoding/json`) with zero new external dependencies.**

Plugins are external executables discovered from `~/.niyantra/plugins/*/plugin.json`. On each poll cycle, Niyantra spawns the plugin as a subprocess, sends a JSON request via stdin, and reads a JSON response from stdout. The plugin process exits after each invocation (short-lived, not a daemon).

## Alternatives Considered

### 1. Go Native Plugin (`-buildmode=plugin`)

**How it works:** Go's built-in `plugin` package loads `.so` shared libraries at runtime.

**Why rejected:**
- ❌ **Does not work on Windows** — Niyantra has Windows users
- ❌ Requires host and plugin compiled with identical Go version + dependency tree
- ❌ Plugins cannot be unloaded once loaded
- ❌ Widely considered a "failed experiment" in the Go community
- ❌ No practical cross-platform distribution model

### 2. HashiCorp go-plugin (gRPC/RPC)

**How it works:** The industry standard for Go plugin systems. Spawns plugins as separate processes and communicates via gRPC or net/rpc. Used by Terraform, Vault, Packer, and Nomad.

**Why rejected:**
- ❌ Adds gRPC + protobuf to the dependency chain (+5–8 MB binary increase)
- ❌ Plugin authors must implement gRPC interfaces — high barrier for script writers
- ❌ Designed for complex, long-running plugin services (Terraform providers)
- ❌ Massive architectural overhead for our simple use case (periodic JSON capture)
- ⚠️ Would increase direct dependency count from 2 to 4+

**When it would be right:** If we needed bidirectional streaming, complex plugin lifecycle management, or plugins written primarily in Go.

### 3. WebAssembly / wazero

**How it works:** Compiles plugins to `.wasm` binaries and runs them in a sandboxed WASM runtime (wazero is pure Go, no CGo).

**Why rejected:**
- ⚠️ Adds wazero as new dependency (~2 MB binary increase, 50% increase in direct deps)
- ❌ Plugin authors need WASM toolchain (TinyGo, Rust target, etc.) — high friction
- ❌ Data marshaling between Go and WASM guest is complex
- ❌ Sandboxing is overkill — our plugins run once every 5 min and exit
- ❌ Cannot easily make HTTP calls from within WASM without host function bridging

**When it would be right:** If we needed to run untrusted third-party code with strong sandboxing guarantees, or if plugins were compute-intensive and needed near-native performance.

### 4. Extism (WASM Framework)

**How it works:** Higher-level framework built on wazero that provides SDKs for both host and plugin development.

**Why rejected:**
- ❌ Two new dependencies (extism-sdk + wazero)
- ❌ Same WASM toolchain friction as #3
- ⚠️ Adds unnecessary abstraction layer over wazero

### 5. Embedded Lua (gopher-lua)

**How it works:** Embeds a Lua 5.1 interpreter in the Go binary. Plugins are `.lua` scripts executed in-process.

**Why rejected:**
- ⚠️ Adds gopher-lua dependency (~1 MB)
- ❌ Users must learn Lua — unfamiliar to most AI/dev tool users
- ❌ Cannot easily make HTTP calls, parse files, or run subprocesses from Lua sandbox
- ❌ In-process execution — a bad Lua script can hang the entire Go runtime
- ❌ No process isolation

**When it would be right:** If plugins were simple data transformation/filtering logic (not I/O-heavy data collection).

### 6. Starlark (go.starlark.net)

**How it works:** Google's deterministic, sandboxed Python-subset language used in Bazel.

**Why rejected:**
- ❌ **Intentionally cannot do I/O** — no HTTP calls, no file reads, no subprocess execution
- ❌ Designed for configuration and build rules, not data collection
- Our plugins fundamentally need to call external APIs, read files, and parse logs

### 7. Yaegi (Go Interpreter)

**How it works:** Traefik's Go interpreter that executes Go source code at runtime without compilation.

**Why rejected:**
- ⚠️ Adds yaegi dependency (~2 MB)
- ❌ Interpreted Go is significantly slower than compiled
- ❌ No sandboxing — full access to Go stdlib including os, net, etc.
- ❌ Our target users write Python/Bash/Node, not Go

### 8. Caddy-style Compile-Time Modules

**How it works:** Plugins are Go packages imported at compile time. Users recompile the binary with their plugins included.

**Why rejected:**
- ❌ Requires users to have Go toolchain installed
- ❌ Breaks single-binary distribution model — each user gets a different binary
- ❌ Cannot add/remove plugins without recompiling

### 9. Prometheus Textfile Collector Pattern

**How it works:** External scripts write metric files to a directory; the host reads them on a schedule.

**Why rejected:**
- ⚠️ No configuration management — scripts manage their own config
- ❌ No structured plugin discovery (just `.prom` files in a directory)
- ❌ File-based communication is less reliable than stdin/stdout (partial writes, stale files)
- ⚠️ Would work for metrics but doesn't support plugin metadata, config schemas, or validation

## Chosen Approach: Telegraf-Inspired Subprocess Exec

### How Telegraf Does It

Telegraf (InfluxData's metrics collection agent) provides two models for external plugins:

- **`inputs.exec`** — Spawns a new subprocess per collection interval, reads stdout
- **`inputs.execd`** — Keeps a long-running daemon process, signals it per interval

We adopt the **`exec` model** (short-lived subprocess) because:
- Simpler lifecycle management (no daemon health monitoring)
- Process isolation is automatic (each invocation is a fresh process)
- No state leaks between invocations
- Timeout enforcement via `exec.CommandContext` is trivial

### Protocol Design

**Discovery:** Scan `~/.niyantra/plugins/*/plugin.json` at startup and on-demand via API.

**Manifest (`plugin.json`):**
```json
{
  "id": "openrouter-usage",
  "name": "OpenRouter Usage Tracker",
  "version": "1.0.0",
  "description": "Tracks OpenRouter API credit usage",
  "author": "user",
  "entryPoint": "capture.py",
  "timeout": 30,
  "capabilities": ["capture"],
  "config": {
    "api_key": { "type": "string", "label": "API Key", "required": true, "secret": true }
  }
}
```

**Input (stdin → plugin):**
```json
{"action": "capture", "config": {"api_key": "sk-or-..."}}
```

**Output (plugin → stdout):**
```json
{
  "status": "ok",
  "data": {
    "provider": "openrouter", "label": "OpenRouter",
    "usage_pct": 45.2, "usage_display": "$4.52 / $10.00",
    "plan": "Pay-as-you-go", "models": [], "metadata": {}
  }
}
```

### Security Model

| Threat | Mitigation |
|--------|-----------|
| Malicious plugin code | Process isolation via `os/exec` — plugin cannot access Niyantra memory |
| Infinite loop / hang | `exec.CommandContext` enforces configurable timeout (default 30s) |
| Secrets exposure | Config values stored in SQLite, masked in API responses (existing pattern) |
| Path traversal | Entry point path validated to be within plugin directory |
| Untrusted output | JSON parsed into strict Go structs; unknown fields ignored |
| Resource exhaustion | Plugins run sequentially (not parallel) in poll loop |

## Consequences

### Positive

- **Zero new dependencies** — binary size unchanged, go.mod unchanged
- **Any language** — users write plugins in Python, Bash, Node, Go, Rust, or any executable
- **Process isolation** — plugin crash cannot crash Niyantra
- **Cross-platform** — `os/exec` works on Windows, Linux, and macOS
- **Low barrier** — writing a plugin is "write a script that reads JSON from stdin and writes JSON to stdout"
- **Fits existing architecture** — plugs into agent polling loop, data_sources table, and notification engine
- **Future-proof** — if we ever need WASM or go-plugin, the JSON protocol can be adapted as a transport layer

### Negative

- **Process startup overhead** — spawning a new process per poll cycle (every 30s–5min). Acceptable for our use case; Telegraf's `exec` plugin works this same way at 10s intervals.
- **No sandboxing** — plugins can do anything the OS user can do (read files, make network calls, etc.). This is acceptable because plugins are user-installed, local-first, and run on the user's own machine.
- **No plugin registry/marketplace** — users must manually create plugin directories. A future feature could add a community plugin index.

### Neutral

- Plugin configuration is stored in Niyantra's SQLite config table (not in the plugin directory), ensuring persistence across plugin updates.
- Plugin snapshots are stored in a dedicated `plugin_snapshots` table, separate from built-in provider tables.

## References

- [Telegraf exec input plugin](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/exec)
- [Telegraf execd input plugin](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/execd)
- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin)
- [wazero WebAssembly runtime](https://wazero.io/)
- [Extism plugin framework](https://extism.org/)
- [gopher-lua](https://github.com/yuin/gopher-lua)
- [Starlark Go implementation](https://pkg.go.dev/go.starlark.net/starlark)
- [Yaegi Go interpreter](https://github.com/traefik/yaegi)
- [Prometheus textfile collector](https://github.com/prometheus/node_exporter#textfile-collector)
