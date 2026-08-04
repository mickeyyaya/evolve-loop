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
	if cfg == nil || cfg.Worktree != "" {
		return
	}
	cfg.Worktree = ScratchCwd(cfg.Workspace, "bridge-scratch-cwd")
}

// ScratchCwd is the single source for "no worktree ⇒ a disposable, owned,
// isolated working directory": it MkdirAll's name under workspace and returns
// the absolute path, or "" when no workspace is owned or the directory cannot
// be created — the caller then keeps its own prior fallback rather than
// aborting. Reaped with the workspace, so it never leaks.
//
// applyScratchCwd above is the probe-launch policy over it. The other consumer
// is the retro phase (phases/retro/retro.go), which under a fleet supervisor
// must hand the bridge a real cwd or its launch is refused outright by
// errWorktreeRequired. Both need the same directory; only one implementation of
// "mint it" may exist, or the two drift on the leak-safety properties (owned
// dir, never main, never process cwd) that are the whole point.
func ScratchCwd(workspace, name string) string {
	if workspace == "" || name == "" {
		return ""
	}
	dir := filepath.Join(workspace, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	return dir
}
