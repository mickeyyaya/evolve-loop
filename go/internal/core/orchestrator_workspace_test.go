package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestArchivePollutedWorkspace_MissingDirIsNoOp covers the happy case
// of a fresh cycle: workspace doesn't exist yet, nothing to archive.
func TestArchivePollutedWorkspace_MissingDirIsNoOp(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "cycle-108")
	now := func() time.Time { return time.Unix(1700000000, 0).UTC() }
	if err := archivePollutedWorkspace(ws, now); err != nil {
		t.Fatalf("missing dir must be no-op, got %v", err)
	}
	if _, err := os.Stat(ws + ".polluted-*"); !os.IsNotExist(err) {
		// Glob form not used; explicit check via ReadDir of parent:
		entries, _ := os.ReadDir(filepath.Dir(ws))
		for _, e := range entries {
			if strings.Contains(e.Name(), "polluted") {
				t.Errorf("missing workspace must not create archive; found %s", e.Name())
			}
		}
	}
}

// TestArchivePollutedWorkspace_EmptyDirIsNoOp covers an empty
// pre-existing dir (e.g., from a mkdir-only init that crashed before
// any phase wrote anything). Treated as fresh.
func TestArchivePollutedWorkspace_EmptyDirIsNoOp(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "cycle-108")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	now := func() time.Time { return time.Unix(1700000000, 0).UTC() }
	if err := archivePollutedWorkspace(ws, now); err != nil {
		t.Fatalf("empty dir must be no-op, got %v", err)
	}
	if _, err := os.Stat(ws); err != nil {
		t.Errorf("empty workspace should still exist: %v", err)
	}
}

// TestArchivePollutedWorkspace_NonEmptyDirRenamed is the regression for
// cycle-108: a prior attempt's scout-report.md must be moved aside so
// the fresh phases run cleanly.
func TestArchivePollutedWorkspace_NonEmptyDirRenamed(t *testing.T) {
	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-108")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("leftover from cycle-108 attempt 1"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	now := func() time.Time { return time.Date(2026, 5, 26, 11, 22, 33, 444555666, time.UTC) }
	if err := archivePollutedWorkspace(ws, now); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// Original path should no longer exist (or be empty)
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Errorf("workspace path should be moved away; stat err=%v", err)
	}
	// Archived sibling should exist with the timestamped name
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("readdir parent: %v", err)
	}
	var archive string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "cycle-108.polluted-") {
			archive = e.Name()
		}
	}
	if archive == "" {
		t.Fatalf("expected polluted archive, got entries=%v", entries)
	}
	// Archive content preserved
	data, err := os.ReadFile(filepath.Join(parent, archive, "scout-report.md"))
	if err != nil {
		t.Fatalf("archived file unreadable: %v", err)
	}
	if string(data) != "leftover from cycle-108 attempt 1" {
		t.Errorf("archive corrupted content: %q", data)
	}
}

// TestArchivePollutedWorkspace_NotADirReturnsNoOp covers the
// pathological case of a regular file at the workspace path. The
// guard treats it as "nothing to archive" and lets downstream code
// either fail loudly or overwrite — we deliberately don't try to
// fix that here.
func TestArchivePollutedWorkspace_NotADirReturnsNoOp(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "cycle-108")
	if err := os.WriteFile(ws, []byte("a file, not a dir"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	now := func() time.Time { return time.Unix(1700000000, 0).UTC() }
	if err := archivePollutedWorkspace(ws, now); err != nil {
		t.Errorf("non-dir should not error, got %v", err)
	}
}
