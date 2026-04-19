//go:build !windows

package client

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// detectProcess finds the language server on macOS / Linux by scanning
// the process table for Antigravity-related command lines.
func (c *Client) detectProcess(ctx context.Context) (*processInfo, error) {
	out, err := exec.CommandContext(ctx, "ps", "aux").Output()
	if err != nil {
		return nil, ErrProcessNotFound
	}

	var best *processInfo
	topRank := -1

	for _, line := range strings.Split(string(out), "\n") {
		if !containsFold(line, "antigravity") {
			continue
		}

		// Ignore installer artefacts
		if containsFold(line, "server installation script") {
			continue
		}

		// Require at least one language-server indicator
		indicators := []string{
			"language-server", "language_server",
			"--csrf_token", "--extension_server_port",
			"exa.language_server_pb", "lsp",
		}
		found := false
		for _, ind := range indicators {
			if strings.Contains(line, ind) {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		p, err := parsePSLine(line)
		if err != nil {
			continue
		}

		if r := rankCandidate(p); r > topRank {
			best = p
			topRank = r
		}
	}

	if best == nil {
		return nil, ErrProcessNotFound
	}
	return best, nil
}

// parsePSLine extracts process info from a single "ps aux" output line.
//
//	Fields: USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND...
func parsePSLine(line string) (*processInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 11 {
		return nil, ErrProcessNotFound
	}

	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, ErrProcessNotFound
	}

	cmdLine := strings.Join(fields[10:], " ")
	return &processInfo{
		PID:                 pid,
		CSRFToken:           parseFlag(cmdLine, "--csrf_token"),
		ExtensionServerPort: parseFlagInt(cmdLine, "--extension_server_port"),
		CommandLine:         cmdLine,
	}, nil
}

// discoverPorts returns TCP ports the given PID is listening on.
// It tries lsof (macOS/Linux), then ss (Linux), then netstat (fallback).
func (c *Client) discoverPorts(ctx context.Context, pid int) ([]int, error) {
	if ports := portsFromLsof(ctx, pid); len(ports) > 0 {
		return ports, nil
	}
	if ports := portsFromSS(ctx, pid); len(ports) > 0 {
		return ports, nil
	}
	return portsFromNetstat(ctx, pid), nil
}

// portsFromLsof uses lsof to list TCP LISTEN ports for a PID.
func portsFromLsof(ctx context.Context, pid int) []int {
	out, err := exec.CommandContext(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN",
		"-a", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	return extractPorts(string(out), "", func(line string) bool {
		return strings.Contains(line, "(LISTEN)")
	})
}

// portsFromSS uses the ss utility (Linux) to list ports for a PID.
func portsFromSS(ctx context.Context, pid int) []int {
	out, err := exec.CommandContext(ctx, "ss", "-tlnp").Output()
	if err != nil {
		return nil
	}
	needle := fmt.Sprintf("pid=%d,", pid)
	return extractPorts(string(out), needle, nil)
}

// portsFromNetstat uses netstat as a final fallback.
func portsFromNetstat(ctx context.Context, pid int) []int {
	out, err := exec.CommandContext(ctx, "netstat", "-tlnp").Output()
	if err != nil {
		return nil
	}
	needle := fmt.Sprintf("%d/", pid)
	return extractPorts(string(out), needle, nil)
}

// extractPorts scans output lines for port numbers.
// If needle is non-empty, only lines containing needle are considered.
// If filter is non-nil, it provides an additional per-line predicate.
func extractPorts(output, needle string, filter func(string) bool) []int {
	var ports []int
	for _, line := range strings.Split(output, "\n") {
		if needle != "" && !strings.Contains(line, needle) {
			continue
		}
		if filter != nil && !filter(line) {
			continue
		}
		// Find ":PORT " pattern — last colon before a space
		for i := len(line) - 1; i >= 0; i-- {
			if line[i] == ' ' || line[i] == '\t' {
				continue
			}
			// Walk back to find the colon
			end := i + 1
			start := strings.LastIndex(line[:end], ":")
			if start < 0 {
				break
			}
			portStr := line[start+1 : end]
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				ports = append(ports, p)
			}
			break
		}
	}
	return ports
}

// containsFold is defined in detect_windows.go on Windows;
// provide the same helper here for non-Windows builds.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
