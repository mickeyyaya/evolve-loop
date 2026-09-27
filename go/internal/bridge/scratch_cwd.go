package bridge

import (
	"os"
	"path/filepath"
)

// applyScratchCwd points a PROBE launch (boot-smoke, health canary) with no
// designated worktree at a disposable directory under its own Workspace,
// rather than the process cwd. No-op when a worktree is already designated or
// no Workspace is owned; a MkdirAll failure leaves Worktree empty so the
// caller degrades to its prior fallback.
func applyScratchCwd(cfg *Config) {
	if cfg == nil || cfg.Worktree != "" {
		return
	}
	cfg.Worktree = ScratchCwd(cfg.Workspace, "bridge-scratch-cwd")
}

// ScratchCwd is the single source for "no worktree ⇒ a disposable, owned,
// isolated working directory": it MkdirAll's name under workspace and
// returns the absolute path, or "" when no workspace is owned or the
// directory cannot be created. applyScratchCwd is the probe-launch policy
// over it; the retro phase is the other consumer.
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
