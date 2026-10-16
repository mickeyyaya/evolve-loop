package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyScratchCwd_PointsProbeAtScratchUnderWorkspace(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := &Config{Workspace: ws}
	applyScratchCwd(cfg)

	want := filepath.Join(ws, "bridge-scratch-cwd")
	if cfg.Worktree != want {
		t.Fatalf("Worktree=%q, want %q (probe must run in scratch, not the process cwd)", cfg.Worktree, want)
	}
	if fi, err := os.Stat(cfg.Worktree); err != nil || !fi.IsDir() {
		t.Fatalf("scratch dir not created: err=%v", err)
	}
	if cwd, _ := os.Getwd(); cfg.Worktree == cwd {
		t.Fatalf("scratch cwd equals process cwd %q — the checkout leak surface is not closed", cwd)
	}
}

func TestScratchCwd_MintsUnderWorkspaceAndFailsOpen(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	got := ScratchCwd(ws, "retro-scratch-cwd")

	want := filepath.Join(ws, "retro-scratch-cwd")
	if got != want {
		t.Fatalf("ScratchCwd = %q, want %q", got, want)
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Fatalf("ScratchCwd did not create the directory: err=%v", err)
	}
	if got := ScratchCwd("", "retro-scratch-cwd"); got != "" {
		t.Errorf("ScratchCwd(no workspace) = %q, want empty — a path outside any owned dir is the leak surface", got)
	}
	if got := ScratchCwd(ws, ""); got != "" {
		t.Errorf("ScratchCwd(no name) = %q, want empty — the workspace root itself is not a disposable scratch dir", got)
	}
}

func TestApplyScratchCwd_NoOpWhenWorktreeSet(t *testing.T) {
	t.Parallel()
	realWorktree := t.TempDir()
	cfg := &Config{Workspace: t.TempDir(), Worktree: realWorktree}
	applyScratchCwd(cfg)
	if cfg.Worktree != realWorktree {
		t.Fatalf("Worktree mutated to %q — a real phase's worktree must be left untouched", cfg.Worktree)
	}
}

func TestApplyScratchCwd_NoOpWhenNoWorkspace(t *testing.T) {
	t.Parallel()
	cfg := &Config{}
	applyScratchCwd(cfg)
	if cfg.Worktree != "" {
		t.Fatalf("Worktree=%q, want empty — no owned Workspace means keep the existing fallback", cfg.Worktree)
	}
}

func TestIsDir_MatchesTheLaunchGuardPredicate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	if !IsDir(dir) {
		t.Fatalf("IsDir(%q) = false for an existing directory — the guard would refuse a live worktree", dir)
	}
	if IsDir(filepath.Join(dir, "torn-down-lane")) {
		t.Fatal("IsDir returned true for a non-existent path — the stale-worktree refusal (cycle-1278) turns on exactly this")
	}
	if IsDir(file) {
		t.Fatalf("IsDir(%q) = true for a regular file — a working dir must be a directory", file)
	}
	if IsDir("") {
		t.Fatal(`IsDir("") = true — the empty worktree shape must not pass as an existing dir`)
	}
}
