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

// === defaultFullDryRunPreflight — script succeeds ============================

// TestDefaultFullDryRunPreflight_ScriptSucceeds: when the script exists,
// is executable, and exits 0, defaultFullDryRunPreflight returns nil.
func TestDefaultFullDryRunPreflight_ScriptSucceeds(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "legacy", "scripts", "release")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	script := filepath.Join(scriptDir, "full-dry-run.sh")
	// Succeeding script: prints version arg and exits 0.
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"dry-run ok $2\"\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := defaultFullDryRunPreflight(dir, "1.2.3")
	if err != nil {
		t.Errorf("defaultFullDryRunPreflight with succeeding script: want nil, got %v", err)
	}
}
