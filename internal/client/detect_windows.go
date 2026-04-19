//go:build windows

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// detectProcess finds the language server on Windows using a cascade of
// detection methods: CIM query → Get-Process → WMIC (legacy).
func (c *Client) detectProcess(ctx context.Context) (*processInfo, error) {
	if p, err := c.findViaCIM(ctx); err == nil {
		return p, nil
	}
	if p, err := c.findViaGetProcess(ctx); err == nil {
		return p, nil
	}
	if p, err := c.findViaWMIC(ctx); err == nil {
		return p, nil
	}
	return nil, ErrProcessNotFound
}

// findViaCIM queries Win32_Process through Get-CimInstance.
func (c *Client) findViaCIM(ctx context.Context) (*processInfo, error) {
	query := `Get-CimInstance Win32_Process -Filter "CommandLine LIKE '%antigravity%' OR Name LIKE '%language_server%'" | ` +
		`Select-Object ProcessId, Name, CommandLine | ConvertTo-Json -Compress`

	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", query).Output()
	if err != nil {
		return nil, fmt.Errorf("CIM lookup failed: %w", err)
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, ErrProcessNotFound
	}

	return c.pickBestCIM(text)
}

// pickBestCIM parses CIM JSON (may be array or single object) and selects
// the highest-ranked candidate.
func (c *Client) pickBestCIM(raw string) (*processInfo, error) {
	type entry struct {
		ProcessId   int    `json:"ProcessId"`
		Name        string `json:"Name"`
		CommandLine string `json:"CommandLine"`
	}

	var entries []entry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		// PowerShell emits a bare object when only one row matches
		var single entry
		if err2 := json.Unmarshal([]byte(raw), &single); err2 != nil {
			return nil, ErrProcessNotFound
		}
		entries = []entry{single}
	}

	var best *processInfo
	topRank := -1

	for _, e := range entries {
		if e.CommandLine == "" || !containsFold(e.CommandLine, "antigravity") {
			continue
		}

		p := &processInfo{
			PID:                 e.ProcessId,
			CSRFToken:           parseFlag(e.CommandLine, "--csrf_token"),
			ExtensionServerPort: parseFlagInt(e.CommandLine, "--extension_server_port"),
			CommandLine:         e.CommandLine,
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

// findViaGetProcess uses Get-Process as a lightweight fallback.
func (c *Client) findViaGetProcess(ctx context.Context) (*processInfo, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		`Get-Process | Where-Object { $_.ProcessName -match 'antigravity|language_server' } | `+
			`Select-Object Id, ProcessName | ConvertTo-Json -Compress`)

	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil, ErrProcessNotFound
	}

	type row struct {
		Id int `json:"Id"`
	}

	var rows []row
	if err := json.Unmarshal(out, &rows); err != nil {
		var single row
		if err2 := json.Unmarshal(out, &single); err2 != nil {
			return nil, ErrProcessNotFound
		}
		rows = []row{single}
	}

	var best *processInfo
	topRank := -1

	for _, r := range rows {
		// Retrieve the full command line for ranking
		clCmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
			fmt.Sprintf(`(Get-CimInstance Win32_Process -Filter "ProcessId = %d").CommandLine`, r.Id))

		clOut, err := clCmd.Output()
		if err != nil {
			continue
		}

		cl := strings.TrimSpace(string(clOut))
		if !containsFold(cl, "antigravity") {
			continue
		}

		p := &processInfo{
			PID:                 r.Id,
			CSRFToken:           parseFlag(cl, "--csrf_token"),
			ExtensionServerPort: parseFlagInt(cl, "--extension_server_port"),
			CommandLine:         cl,
		}

		if rk := rankCandidate(p); rk > topRank {
			best = p
			topRank = rk
		}
	}

	if best == nil {
		return nil, ErrProcessNotFound
	}
	return best, nil
}

// findViaWMIC is a legacy fallback for older Windows builds.
func (c *Client) findViaWMIC(ctx context.Context) (*processInfo, error) {
	cmd := exec.CommandContext(ctx, "wmic", "process", "where",
		"name like '%antigravity%' or commandline like '%antigravity%'",
		"get", "processid,commandline", "/format:csv")

	out, err := cmd.Output()
	if err != nil {
		return nil, ErrProcessNotFound
	}

	var best *processInfo
	topRank := -1

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node,") {
			continue
		}

		cols := strings.Split(line, ",")
		if len(cols) < 3 {
			continue
		}

		cl := strings.Join(cols[1:len(cols)-1], ",")
		if !containsFold(cl, "antigravity") {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(cols[len(cols)-1]))
		if err != nil {
			continue
		}

		p := &processInfo{
			PID:                 pid,
			CSRFToken:           parseFlag(cl, "--csrf_token"),
			ExtensionServerPort: parseFlagInt(cl, "--extension_server_port"),
			CommandLine:         cl,
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

// discoverPorts enumerates listening ports for a given PID on Windows.
func (c *Client) discoverPorts(ctx context.Context, pid int) ([]int, error) {
	out, err := exec.CommandContext(ctx, "netstat", "-ano").Output()
	if err != nil {
		return nil, err
	}

	pidStr := strconv.Itoa(pid)
	var ports []int

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "LISTENING") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// Last field is the PID
		if fields[len(fields)-1] != pidStr {
			continue
		}
		// Local address is field[1], format "0.0.0.0:PORT" or "[::]:PORT"
		addr := fields[1]
		colon := strings.LastIndex(addr, ":")
		if colon < 0 {
			continue
		}
		port, err := strconv.Atoi(addr[colon+1:])
		if err == nil && port > 0 {
			ports = append(ports, port)
		}
	}

	return ports, nil
}

// containsFold reports whether s contains substr, ignoring case.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
