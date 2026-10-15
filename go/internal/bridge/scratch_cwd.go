package bridge

import (
	"os"
	"path/filepath"
)

func applyScratchCwd(cfg *Config) {
	if cfg == nil || cfg.Worktree != "" {
		return
	}
	cfg.Worktree = ScratchCwd(cfg.Workspace, "bridge-scratch-cwd")
}

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
