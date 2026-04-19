package client

import (
	"strconv"
	"strings"
)

// parseFlag extracts a named flag value from a command line string.
// Handles both "--flag=value" and "--flag value" forms.
func parseFlag(cmdLine, flag string) string {
	idx := strings.Index(cmdLine, flag)
	if idx < 0 {
		return ""
	}
	rest := cmdLine[idx+len(flag):]

	// "--flag=value"
	if len(rest) > 0 && rest[0] == '=' {
		return nextToken(rest[1:])
	}

	// "--flag value" (separated by whitespace)
	trimmed := strings.TrimLeft(rest, " \t")
	if len(trimmed) == 0 || trimmed == rest {
		// No whitespace separator found — not a valid form
		return ""
	}
	return nextToken(trimmed)
}

// nextToken returns the first whitespace-delimited token from s,
// stripping surrounding quotes if present.
func nextToken(s string) string {
	if len(s) == 0 {
		return ""
	}
	// Handle quoted values
	if s[0] == '"' || s[0] == '\'' {
		quote := s[0]
		end := strings.IndexByte(s[1:], quote)
		if end >= 0 {
			return s[1 : end+1]
		}
	}
	// Unquoted: take until whitespace
	end := strings.IndexAny(s, " \t\n")
	if end < 0 {
		return s
	}
	return s[:end]
}

// parseFlagInt extracts a named flag as an integer.
func parseFlagInt(cmdLine, flag string) int {
	v := parseFlag(cmdLine, flag)
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

// rankCandidate assigns a priority rank to a detected process.
// Higher rank = more likely to be the real language server.
// Uses a tiered priority scheme rather than point accumulation.
func rankCandidate(p *processInfo) int {
	lower := strings.ToLower(p.CommandLine)
	rank := 0

	// Tier 1: has CSRF token — definitive LS indicator
	if p.CSRFToken != "" {
		rank += 100
	}
	// Tier 2: binary name contains "language_server"
	if strings.Contains(lower, "language_server") ||
		strings.Contains(lower, "language-server") {
		rank += 40
	}
	// Tier 3: references the protobuf service
	if strings.Contains(lower, "exa.language_server_pb") {
		rank += 30
	}
	// Tier 4: has extension server port
	if p.ExtensionServerPort > 0 {
		rank += 15
	}
	// Tier 5: mentions LSP
	if strings.Contains(lower, "lsp") {
		rank += 8
	}
	// Tier 6: mentions antigravity at all
	if strings.Contains(lower, "antigravity") {
		rank += 3
	}

	return rank
}
