package cyclecost

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEventsLog_ExportedDelegatesToCanonical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scout-events.ndjson")
	if err := os.WriteFile(path, []byte(`{"kind":"result","data":{"tokens":{"in":11,"out":22,"cache_r":3,"cache_c":4}}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	exported, okE := ParseEventsLog(path)
	internal, okI := parseEventsLog(path)
	if okE != okI || exported != internal {
		t.Fatalf("ParseEventsLog must delegate to the canonical parser: exported=(%+v,%v) internal=(%+v,%v)", exported, okE, internal, okI)
	}
	if exported.InputTokens != 11 || exported.OutputTokens != 22 {
		t.Errorf("token counts not recovered: got %+v", exported)
	}
}

func TestSummarizeCycle_StatPermissionError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod doesn't restrict")
	}
	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-1")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(parent, 0o755)

	_, err := SummarizeCycle(ws, 1)
	if err == nil {
		t.Fatalf("expected error with parent chmod 000")
	}
}

func TestParseEventsLog_OpenError(t *testing.T) {
	t.Parallel()
	_, ok := parseEventsLog(filepath.Join(t.TempDir(), "does-not-exist-events.ndjson"))
	if ok {
		t.Fatalf("expected ok=false for missing file")
	}
}

func TestParseEventsLog_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty-events.ndjson")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, ok := parseEventsLog(path)
	if ok {
		t.Fatalf("empty file should yield ok=false")
	}
}

func TestParseEventsLog_OnlyBlankLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "blank-events.ndjson")
	if err := os.WriteFile(path, []byte("\n\n   \n\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, ok := parseEventsLog(path)
	if ok {
		t.Fatalf("blank-only file should yield ok=false")
	}
}

func TestParseEventsLog_MalformedAfterPreFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "broken-events.ndjson")
	body := `{garbage "kind":"result" still garbage}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, ok := parseEventsLog(path)
	if ok {
		t.Fatalf("malformed-after-prefilter should yield ok=false")
	}
}

func TestParseEventsLog_ScannerErrLineTooLong(t *testing.T) {
	prev := maxScannerBufBytes
	defer func() { maxScannerBufBytes = prev }()
	maxScannerBufBytes = 1024

	dir := t.TempDir()
	path := filepath.Join(dir, "huge-events.ndjson")
	body := strings.Repeat("x", 2048) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, ok := parseEventsLog(path)
	if ok {
		t.Fatalf("expected ok=false on scanner err")
	}
}

func TestSummarizeCycle_GlobError(t *testing.T) {
	prev := globFn
	defer func() { globFn = prev }()
	globFn = func(string) ([]string, error) { return nil, errors.New("synthetic glob") }

	ws := filepath.Join(t.TempDir(), "cycle-1")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := SummarizeCycle(ws, 1)
	if err == nil {
		t.Fatalf("expected glob error")
	}
}
