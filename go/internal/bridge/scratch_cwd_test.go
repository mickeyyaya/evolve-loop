package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

// I1: a PROBE launch (boot-smoke, model-query, health canary) with no designated
// worktree must run in a disposable scratch dir under its own Workspace — never
// the process cwd (the live checkout), where a non-Claude CLI could write into
// main. applyScratchCwd is the shared helper the probe call sites use.
func TestApplyScratchCwd_PointsProbeAtScratchUnderWorkspace(t *testing.T) {
	ws := t.TempDir()
	cfg := &Config{Workspace: ws} // probe: no Worktree designated
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

// A real phase already carries its worktree — applyScratchCwd must never touch it
// (the degraded-mode os.Getwd() fallback in runTmuxREPL stays the phase path).
func TestApplyScratchCwd_NoOpWhenWorktreeSet(t *testing.T) {
	realWorktree := t.TempDir()
	cfg := &Config{Workspace: t.TempDir(), Worktree: realWorktree}
	applyScratchCwd(cfg)
	if cfg.Worktree != realWorktree {
		t.Fatalf("Worktree mutated to %q — a real phase's worktree must be left untouched", cfg.Worktree)
	}
}

// With no owned Workspace there is nowhere safe to put a scratch dir, so the
// caller keeps its existing fallback (Worktree stays empty) — no temp leak.
func TestApplyScratchCwd_NoOpWhenNoWorkspace(t *testing.T) {
	cfg := &Config{}
	applyScratchCwd(cfg)
	if cfg.Worktree != "" {
		t.Fatalf("Worktree=%q, want empty — no owned Workspace means keep the existing fallback", cfg.Worktree)
	}
}
