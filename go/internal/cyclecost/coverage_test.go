package cyclecost

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// globFn / lstatFn seams would be cleaner, but tests can drive the
// remaining branches via filesystem manipulation alone.

// TestSummarizeCycle_StatPermissionError covers the os.Stat non-
// NotExist error branch. Achieved by chmod 000 on the workspace's
// parent directory so Stat returns EACCES.
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
	// chmod parent to non-traversable so Stat on the child fails.
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(parent, 0o755)

	_, err := SummarizeCycle(ws, 1)
	// Either ErrNoWorkspace or a wrapped EACCES error is acceptable. The
	// important thing is a non-nil error without panicking.
	if err == nil {
		t.Fatalf("expected error with parent chmod 000")
	}
}

// TestSummarizeCycle_GlobReturnsEmpty exercises the len(logs) == 0 branch
// (workspace exists, no *-events.ndjson files).
func TestSummarizeCycle_GlobReturnsEmpty(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-1")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := SummarizeCycle(ws, 1)
	if !errors.Is(err, ErrNoLogs) {
		t.Fatalf("err=%v want ErrNoLogs", err)
	}
}

// TestParseEventsLog_OpenError covers the os.Open error branch directly.
func TestParseEventsLog_OpenError(t *testing.T) {
	t.Parallel()
	_, ok := parseEventsLog(filepath.Join(t.TempDir(), "does-not-exist-events.ndjson"))
	if ok {
		t.Fatalf("expected ok=false for missing file")
	}
}

// TestParseEventsLog_EmptyFile covers the no-result-found branch.
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

// TestParseEventsLog_OnlyBlankLines covers blank-line-only input.
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

// TestParseEventsLog_MalformedAfterPreFilter covers the
// `json.Unmarshal(line, &ev); err != nil { continue }` branch — a line that
// contains the `"kind":"result"` substring but is not valid JSON.
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

// TestParseEventsLog_ScannerErrLineTooLong covers the scanner.Err()
// branch (bufio.ErrTooLong) by lowering the buffer cap.
func TestParseEventsLog_ScannerErrLineTooLong(t *testing.T) {
	// NOT t.Parallel — mutates package-level maxScannerBufBytes.
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

// TestSummarizeCycle_GlobError covers the filepath.Glob error branch
// via the globFn seam.
func TestSummarizeCycle_GlobError(t *testing.T) {
	// NOT t.Parallel — mutates package-level globFn.
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
