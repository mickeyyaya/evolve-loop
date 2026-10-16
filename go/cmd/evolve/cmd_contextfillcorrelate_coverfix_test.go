package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextFillCoverfixWriteReportStatErrorIsWrapped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "plain")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeReportAtomic(filepath.Join(file, "report.md"), "# r\n")
	if err == nil || !strings.Contains(err.Error(), "stat ") {
		t.Fatalf("err=%v, want a wrapped stat failure", err)
	}
}

func TestContextFillCoverfixWriteReportTempWriteFailureReturned(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing-dir", "report.md")
	if err := writeReportAtomic(path, "# r\n"); err == nil {
		t.Fatal("want an error when the temp file cannot be created")
	}
}

func TestContextFillCoverfixWriteReportRenameFailureCleansTemp(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "report.md")
	boom := errors.New("rename boom")
	err := writeReportAtomicWith(func(string, string) error { return boom }, path, "# r\n")
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v, want the rename error", err)
	}
	if _, serr := os.Stat(path + ".tmp"); !os.IsNotExist(serr) {
		t.Errorf("temp file must be removed after rename failure (stat err=%v)", serr)
	}
	if _, serr := os.Stat(path); !os.IsNotExist(serr) {
		t.Errorf("report must not exist after rename failure (stat err=%v)", serr)
	}
}
