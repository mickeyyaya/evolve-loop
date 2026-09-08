package bridge

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
)

func sandboxTerminalPath(ctx context.Context, deps Deps, cfg *Config, session string) (string, error) {
	if runtime.GOOS != "darwin" || cfg.Worktree == "" && !cfg.RequireSandbox || strings.TrimSpace(deps.Env[envSandboxMode]) == "off" {
		return "", nil
	}
	reader, ok := deps.Tmux.(paneTTYReader)
	if !ok {
		return "", nil
	}
	path, err := reader.paneTTY(ctx, session)
	if err != nil {
		return "", err
	}
	if err := validateSandboxTerminal(path); err != nil {
		return "", err
	}
	return path, nil
}

func validateSandboxTerminal(path string) error {
	suffix, ok := strings.CutPrefix(path, "/dev/ttys")
	if !ok || suffix == "" {
		return fmt.Errorf("invalid assigned macOS terminal %q", path)
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid assigned macOS terminal %q", path)
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("assigned terminal: %w", err)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("assigned terminal %q is not a character device", path)
	}
	return nil
}
