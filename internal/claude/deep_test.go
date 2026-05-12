package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-05-12T10:03:54.506Z", "2026-05-12"},
		{"2026-02-05T10:01:34.646Z", "2026-02-05"},
		{"", ""},
		{"short", ""},
		{"2026-01-01", "2026-01-01"},
	}

	for _, tc := range tests {
		got := extractDate(tc.input)
		if got != tc.want {
			t.Errorf("extractDate(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeModel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-sonnet-4-20250514", "claude-sonnet-4"},
		{"claude-opus-4-20250514", "claude-opus-4"},
		{"gpt-oss:20b", "gpt-oss:20b"},
		{"claude-haiku-3.5-20241022", "claude-haiku-3.5"},
		{"", "unknown"},
	}

	for _, tc := range tests {
		got := normalizeModel(tc.input)
		if got != tc.want {
			t.Errorf("normalizeModel(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseSessionFile(t *testing.T) {
	// Create a temporary JSONL fixture
	dir := t.TempDir()
	jsonlPath := filepath.Join(dir, "test-session.jsonl")

	content := `{"type":"system","subtype":"turn_duration","durationMs":66435,"timestamp":"2026-05-12T10:01:00.000Z"}
{"type":"user","message":{"role":"user","content":"hello"},"timestamp":"2026-05-12T10:01:05.000Z","sessionId":"sess-001"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":1500,"output_tokens":300,"cache_read_input_tokens":500,"cache_creation_input_tokens":100}},"timestamp":"2026-05-12T10:01:10.000Z","sessionId":"sess-001"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":2000,"output_tokens":800,"cache_read_input_tokens":1000,"cache_creation_input_tokens":0}},"timestamp":"2026-05-12T10:02:00.000Z","sessionId":"sess-001"}
{"type":"assistant","message":{"model":"claude-opus-4-20250514","usage":{"input_tokens":5000,"output_tokens":2000,"cache_read_input_tokens":0,"cache_creation_input_tokens":500}},"timestamp":"2026-05-12T10:03:00.000Z","sessionId":"sess-001"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20250514","content":[{"type":"thinking"}],"usage":{"input_tokens":0,"output_tokens":0}},"timestamp":"2026-05-12T10:04:00.000Z","sessionId":"sess-001"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	usages, err := ParseSessionFile(jsonlPath)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}

	// Should find 3 assistant turns with non-zero tokens (4th has 0/0, skipped)
	if len(usages) != 3 {
		t.Fatalf("expected 3 usage records, got %d", len(usages))
	}

	// First record: sonnet, 1500 input, 300 output
	if usages[0].InputTokens != 1500 {
		t.Errorf("first record input: expected 1500, got %d", usages[0].InputTokens)
	}
	if usages[0].OutputTokens != 300 {
		t.Errorf("first record output: expected 300, got %d", usages[0].OutputTokens)
	}
	if usages[0].CacheReadTokens != 500 {
		t.Errorf("first record cache read: expected 500, got %d", usages[0].CacheReadTokens)
	}
	if usages[0].Model != "claude-sonnet-4-20250514" {
		t.Errorf("first record model: expected claude-sonnet-4-20250514, got %s", usages[0].Model)
	}

	// Third record: opus, 5000 input
	if usages[2].InputTokens != 5000 {
		t.Errorf("third record input: expected 5000, got %d", usages[2].InputTokens)
	}
	if usages[2].Model != "claude-opus-4-20250514" {
		t.Errorf("third record model: expected claude-opus-4-20250514, got %s", usages[2].Model)
	}
}

func TestParseSessionFile_Empty(t *testing.T) {
	dir := t.TempDir()
	jsonlPath := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(jsonlPath, []byte(""), 0644)

	usages, err := ParseSessionFile(jsonlPath)
	if err != nil {
		t.Fatalf("ParseSessionFile empty: %v", err)
	}
	if len(usages) != 0 {
		t.Errorf("expected 0 usages from empty file, got %d", len(usages))
	}
}

func TestParseSessionFile_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	jsonlPath := filepath.Join(dir, "bad.jsonl")

	content := `this is not json
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
also not json {{{
`
	os.WriteFile(jsonlPath, []byte(content), 0644)

	usages, err := ParseSessionFile(jsonlPath)
	if err != nil {
		t.Fatalf("ParseSessionFile malformed: %v", err)
	}
	// Should extract 1 valid record, skip malformed
	if len(usages) != 1 {
		t.Errorf("expected 1 usage from malformed file, got %d", len(usages))
	}
}

func TestFindSessionFiles_NoDir(t *testing.T) {
	// This tests graceful degradation when projects dir doesn't exist
	// We can't override HOME easily, so just verify it doesn't panic
	files, err := FindSessionFiles()
	if err != nil {
		t.Fatalf("FindSessionFiles: %v", err)
	}
	// May find files on the test system or may not — just no crash
	_ = files
}

func TestAggregateUsage_WithFixtures(t *testing.T) {
	// Create a mock project structure
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "test-project", "sessions")
	os.MkdirAll(projectDir, 0755)

	// Session 1: 2 turns on same day
	s1 := `{"type":"assistant","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":1000,"output_tokens":200,"cache_read_input_tokens":500,"cache_creation_input_tokens":100}},"timestamp":"2026-05-12T10:00:00.000Z","sessionId":"sess-001"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":2000,"output_tokens":400,"cache_read_input_tokens":1000,"cache_creation_input_tokens":0}},"timestamp":"2026-05-12T11:00:00.000Z","sessionId":"sess-001"}
`
	s1Path := filepath.Join(projectDir, "sess-001.jsonl")
	os.WriteFile(s1Path, []byte(s1), 0644)

	// Session 2: 1 turn on different day
	s2 := `{"type":"assistant","message":{"model":"claude-opus-4-20250514","usage":{"input_tokens":5000,"output_tokens":1000,"cache_read_input_tokens":0,"cache_creation_input_tokens":500}},"timestamp":"2026-05-11T14:00:00.000Z","sessionId":"sess-002"}
`
	s2Path := filepath.Join(projectDir, "sess-002.jsonl")
	os.WriteFile(s2Path, []byte(s2), 0644)

	// Parse just these files directly (bypassing FindSessionFiles which uses HOME)
	var allUsages []TokenUsage
	for _, path := range []string{s1Path, s2Path} {
		usages, err := ParseSessionFile(path)
		if err != nil {
			t.Fatalf("ParseSessionFile(%s): %v", path, err)
		}
		allUsages = append(allUsages, usages...)
	}

	if len(allUsages) != 3 {
		t.Fatalf("expected 3 total usage records, got %d", len(allUsages))
	}

	// Verify total tokens
	var totalInput, totalOutput int64
	for _, u := range allUsages {
		totalInput += u.InputTokens
		totalOutput += u.OutputTokens
	}
	if totalInput != 8000 {
		t.Errorf("expected total input 8000, got %d", totalInput)
	}
	if totalOutput != 1600 {
		t.Errorf("expected total output 1600, got %d", totalOutput)
	}
}
