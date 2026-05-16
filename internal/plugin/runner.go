package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Run executes a plugin's capture action via subprocess.
// The plugin receives a JSON request on stdin and must write a JSON response to stdout.
// Uses exec.CommandContext for timeout enforcement — the process is killed if it
// exceeds the manifest's timeout (default 30s).
func (p *Plugin) Run(ctx context.Context, logger *slog.Logger) (*CaptureResult, error) {
	timeout := time.Duration(p.Manifest.EffectiveTimeout()) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Determine how to invoke the entry point.
	// For scripts (.py, .sh, .js, etc.), we need an interpreter prefix.
	// For compiled binaries, we invoke directly.
	cmd := p.buildCommand(ctx)
	cmd.Dir = p.Dir

	// Set up stdin pipe for sending the request
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin %s: stdin pipe: %w", p.Manifest.ID, err)
	}

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("plugin %s: start: %w", p.Manifest.ID, err)
	}

	// Write the capture request to stdin, then close to signal EOF
	request := CaptureRequest{
		Action: "capture",
		Config: p.Config,
	}
	go func() {
		defer stdin.Close()
		json.NewEncoder(stdin).Encode(request)
	}()

	// Wait for process completion
	waitErr := cmd.Wait()

	// Log stderr output if any (plugin diagnostic messages)
	if stderrBuf.Len() > 0 {
		// Truncate long stderr to avoid log spam
		stderr := strings.TrimSpace(stderrBuf.String())
		if len(stderr) > 500 {
			stderr = stderr[:500] + "..."
		}
		logger.Debug("Plugin stderr", "plugin", p.Manifest.ID, "stderr", stderr)
	}

	// Check for context timeout
	if ctx.Err() != nil {
		return nil, fmt.Errorf("plugin %s: timed out after %s", p.Manifest.ID, timeout)
	}

	// Check for process exit error
	if waitErr != nil {
		stderr := strings.TrimSpace(stderrBuf.String())
		if len(stderr) > 200 {
			stderr = stderr[:200] + "..."
		}
		return nil, fmt.Errorf("plugin %s: exited with error: %w (stderr: %s)", p.Manifest.ID, waitErr, stderr)
	}

	// Parse JSON response from stdout
	output := strings.TrimSpace(stdoutBuf.String())
	if output == "" {
		return nil, fmt.Errorf("plugin %s: empty stdout (no JSON response)", p.Manifest.ID)
	}

	var result CaptureResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		// Truncate raw output for error message
		preview := output
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("plugin %s: invalid JSON response: %w (raw: %s)", p.Manifest.ID, err, preview)
	}

	return &result, nil
}

// buildCommand creates the exec.Cmd for running this plugin.
// It auto-detects the interpreter based on file extension.
func (p *Plugin) buildCommand(ctx context.Context) *exec.Cmd {
	entry := p.EntryPath
	ext := strings.ToLower(filepath.Ext(entry))

	switch ext {
	case ".py":
		// Try python3 first, fall back to python
		if _, err := exec.LookPath("python3"); err == nil {
			return exec.CommandContext(ctx, "python3", entry)
		}
		return exec.CommandContext(ctx, "python", entry)
	case ".js":
		return exec.CommandContext(ctx, "node", entry)
	case ".ts":
		// Support ts-node, deno, bun for TypeScript
		if _, err := exec.LookPath("deno"); err == nil {
			return exec.CommandContext(ctx, "deno", "run", "--allow-net", "--allow-read", "--allow-env", entry)
		}
		if _, err := exec.LookPath("bun"); err == nil {
			return exec.CommandContext(ctx, "bun", "run", entry)
		}
		return exec.CommandContext(ctx, "npx", "ts-node", entry)
	case ".sh":
		return exec.CommandContext(ctx, "bash", entry)
	case ".ps1":
		return exec.CommandContext(ctx, "pwsh", "-File", entry)
	case ".rb":
		return exec.CommandContext(ctx, "ruby", entry)
	default:
		// Assume compiled binary or has shebang
		return exec.CommandContext(ctx, entry)
	}
}
