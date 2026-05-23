package cycleclassify

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestClassify_LogReadError covers the os.ReadFile error branch in the
// per-role log scan loop. Achieved by writing a log file then making
// it unreadable. On the rare CI environment where chmod doesn't
// restrict the test process (root), the test skips.
func TestClassify_LogReadError(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("OK"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	logPath := filepath.Join(ws, "builder-stdout.log")
	if err := os.WriteFile(logPath, []byte("EPERM"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := os.Chmod(logPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(logPath, 0o644)

	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}

	r := Classify(ws)
	// With log unreadable, the EPERM marker is invisible. The report
	// itself has no markers, so classification falls through to
	// integrity-breach. This proves the `continue` branch was taken.
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("expected integrity-breach when log unreadable; got %s", r.Class)
	}
}

// TestClassify_GlobError covers the filepath.Glob error branches in
// listLogs. Real Glob with literal patterns can't fail, so we swap
// globFn to inject the error.
//
// NOT t.Parallel: mutates the package-level globFn variable.
func TestClassify_GlobError(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("OK"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Save and restore globFn. listLogs swallows the error and Classify
	// falls through to integrity-breach (since report has no markers).
	prev := globFn
	defer func() { globFn = prev }()
	globFn = func(string) ([]string, error) { return nil, errors.New("synthetic glob error") }

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("expected integrity-breach on glob error; got %s", r.Class)
	}
}

// TestClassify_GlobErrorOnStderr exercises the SECOND Glob call's
// error branch (stdout glob succeeded, stderr glob failed). globFn is
// patched to fail only on the stderr pattern.
//
// NOT t.Parallel: mutates the package-level globFn variable.
func TestClassify_GlobErrorOnStderr(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("OK"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	prev := globFn
	defer func() { globFn = prev }()
	globFn = func(pat string) ([]string, error) {
		if filepath.Base(pat) == "*-stderr.log" {
			return nil, errors.New("synthetic stderr glob error")
		}
		return filepath.Glob(pat)
	}
	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("expected integrity-breach; got %s", r.Class)
	}
}
