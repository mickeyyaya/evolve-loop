package bridge

import (
	"os"
	"path/filepath"
)

// applyScratchCwd points a PROBE launch (boot-smoke, health canary) at a
// disposable empty working directory under its own Workspace when it has no
// designated worktree. Probes have no isolated tree; without this they fall back
// to os.Getwd() — the live checkout — where a non-Claude CLI (codex/agy, not
// bound by the Claude role-gate, with the OS sandbox off on nested-macOS) can
// write into main (the build-leak / tree-diff-guard class). The scratch dir
// lives under the Workspace, so it is reaped with the Workspace and never leaks.
//
// No-op when a worktree is already designated (a REAL phase — its worktree must
// be left untouched, so the degraded-mode os.Getwd() fallback in runTmuxREPL and
// the recipe path stays exactly as-is for a provisioning failure) or when no
// Workspace is owned (the caller keeps its existing fallback). Best-effort: a
// MkdirAll failure leaves Worktree empty so the caller degrades to its prior
// behavior rather than aborting a probe.
func applyScratchCwd(cfg *Config) {
	if cfg == nil || cfg.Worktree != "" || cfg.Workspace == "" {
		return
	}
	dir := filepath.Join(cfg.Workspace, "bridge-scratch-cwd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	cfg.Worktree = dir
}
