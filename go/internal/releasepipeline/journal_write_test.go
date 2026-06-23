package releasepipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// === writeJournal — WriteFile error branch ===================================

// TestWriteJournal_UnwritableDir: when the parent directory of the journal
// path is not writable, os.WriteFile returns an error and writeJournal
// propagates it.
func TestWriteJournal_UnwritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — file-permission tests are unreliable")
	}
	dir := t.TempDir()
	// Make the directory non-writable so WriteFile fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Restore write permission at cleanup so TempDir cleanup can delete it.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	j := &Journal{Version: "1.2.3", Tag: "v1.2.3", Steps: []StepRecord{}}
	path := filepath.Join(dir, "journal.json")

	err := writeJournal(j, path)
	if err == nil {
		t.Error("writeJournal to unwritable dir: want error, got nil")
	}
}

// === initJournal — writeJournal failure propagates ==========================

// TestInitJournal_WriteJournalFails: when the journal directory exists but is
// not writable, writeJournal fails and initJournal returns the error with the
// path still set.
func TestInitJournal_WriteJournalFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — file-permission tests are unreliable")
	}
	dir := t.TempDir()
	journalDir := filepath.Join(dir, "release-journal")
	if err := os.MkdirAll(journalDir, 0o555); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(journalDir, 0o755) })

	_, path, err := initJournal(Options{
		Target:     "1.2.3",
		RepoRoot:   dir,
		JournalDir: journalDir,
	}, "v1.2.2", time.Now())
	if err == nil {
		t.Error("initJournal with unwritable dir: want error, got nil")
	}
	// path is set even on write failure (the path was computed before the write).
	if path == "" {
		t.Error("initJournal must return the attempted path even on failure")
	}
}

// NOTE: the former TestDefaultFullDryRunPreflight_Script* tests were removed in
// ADR-0062/T1.3 — defaultFullDryRunPreflight no longer shells out to the deleted
// legacy/scripts/release/full-dry-run.sh. Its Go-native behavior is covered by
// TestDefaultFullDryRunPreflight_NoDeadScript (preflight_no_deadscript_test.go).
