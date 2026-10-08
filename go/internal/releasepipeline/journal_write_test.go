package releasepipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteJournal_UnwritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — file-permission tests are unreliable")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	j := &Journal{Version: "1.2.3", Tag: "v1.2.3", Steps: []StepRecord{}}
	path := filepath.Join(dir, "journal.json")

	err := writeJournal(j, path)
	if err == nil {
		t.Error("writeJournal to unwritable dir: want error, got nil")
	}
}

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
	}, time.Now())
	if err == nil {
		t.Error("initJournal with unwritable dir: want error, got nil")
	}
	if path == "" {
		t.Error("initJournal must return the attempted path even on failure")
	}
}
