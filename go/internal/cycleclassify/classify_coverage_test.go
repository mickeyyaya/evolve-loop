package cycleclassify

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("expected integrity-breach when events file unreadable; got %s", r.Class)
	}
}

// Not parallel: mutates the package-level maxScannerBufBytes.
func TestClassify_EventsLineTooLong(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("clean, no markers"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	// The effective cap is max(1024-byte initial buffer, maxScannerBufBytes), so the line must exceed 1024 bytes.
	pad := strings.Repeat("x", 2000)
	line := `{"kind":"infra_failure","data":{"marker":"eperm","pad":"` + pad + `"}}`
	if err := os.WriteFile(filepath.Join(ws, "scout-events.ndjson"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

	prev := maxScannerBufBytes
	defer func() { maxScannerBufBytes = prev }()
	maxScannerBufBytes = 64

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("over-long events line should yield breach, not %s", r.Class)
	}
}

// Not parallel: mutates the package-level globFn.
func TestClassify_GlobError(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("OK"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	prev := globFn
	defer func() { globFn = prev }()
	globFn = func(string) ([]string, error) { return nil, errors.New("synthetic glob error") }

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("expected integrity-breach on glob error; got %s", r.Class)
	}
}
