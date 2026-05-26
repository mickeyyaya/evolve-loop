package cycleclassify

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClassify_EventsReadError covers the os.Open error branch in the
// events-stream scan loop. Achieved by writing an events file then making
// it unreadable. On the rare CI environment where chmod doesn't restrict
// the test process (root), the test skips.
func TestClassify_EventsReadError(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("OK"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	logPath := filepath.Join(ws, "builder-events.ndjson")
	if err := os.WriteFile(logPath, []byte(`{"kind":"infra_failure","data":{"marker":"eperm"}}`), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	if err := os.Chmod(logPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(logPath, 0o644)

	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}

	r := Classify(ws)
	// With the events file unreadable, the infra_failure is invisible. The
	// report itself has no markers, so classification falls through to
	// integrity-breach. This proves the read-error `continue` branch was taken.
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("expected integrity-breach when events file unreadable; got %s", r.Class)
	}
}

// TestClassify_EventsLineTooLong covers the scanner.Err() branch in
// firstInfraMarker: an events line longer than maxScannerBufBytes can't be
// scanned, so its infra_failure signal is not recovered and Classify falls
// through to integrity-breach (the report itself is clean). Mirrors
// cyclecost's shrink-buffer coverage test.
//
// NOT t.Parallel: mutates the package-level maxScannerBufBytes variable.
func TestClassify_EventsLineTooLong(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("clean, no markers"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	// A valid infra_failure envelope, but padded past the scanner cap. The
	// effective cap is max(cap(initialBuf)=1024, maxScannerBufBytes), so the
	// line must exceed 1024 bytes to overflow even with maxScannerBufBytes
	// shrunk.
	pad := strings.Repeat("x", 2000)
	line := `{"kind":"infra_failure","data":{"marker":"eperm","pad":"` + pad + `"}}`
	if err := os.WriteFile(filepath.Join(ws, "scout-events.ndjson"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

	prev := maxScannerBufBytes
	defer func() { maxScannerBufBytes = prev }()
	maxScannerBufBytes = 64 // floored to the 1024-byte initial buffer cap

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("over-long events line should yield breach, not %s", r.Class)
	}
}

// TestClassify_GlobError covers the filepath.Glob error branch in the
// events scan. Real Glob with a literal pattern can't fail, so we swap
// globFn to inject the error.
//
// NOT t.Parallel: mutates the package-level globFn variable.
func TestClassify_GlobError(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("OK"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Save and restore globFn. The scan swallows the error and Classify
	// falls through to integrity-breach (since the report has no markers).
	prev := globFn
	defer func() { globFn = prev }()
	globFn = func(string) ([]string, error) { return nil, errors.New("synthetic glob error") }

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("expected integrity-breach on glob error; got %s", r.Class)
	}
}
