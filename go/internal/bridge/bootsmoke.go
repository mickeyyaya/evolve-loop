package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// BootSmokeTest boots a *-tmux driver through its normal Launch with BootOnly set and exits without a prompt, returning the exit code and final scrollback.
func BootSmokeTest(ctx context.Context, driverName string, cfg *Config, deps Deps) (rc int, scrollback string) {
	d, ok := LookupDriver(driverName)
	if !ok || !strings.HasSuffix(driverName, "-tmux") {
		return ExitBadFlags, ""
	}
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.CLI = driverName
	cfg.BootOnly = true
	cfg.AllowBypass = true // boot-only runs no task; bypass-equivalent so the safety gate passes
	if cfg.Workspace == "" {
		// Own a scratch workspace for a minimal cfg; the deferred removal runs after the scrollback read below.
		tmp, err := os.MkdirTemp("", "evolve-bootsmoke-*")
		if err != nil {
			return ExitBadFlags, ""
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cfg.Workspace = tmp
	}
	// Boot in a scratch dir under the workspace, never the live checkout: an empty Worktree falls back to os.Getwd().
	applyScratchCwd(cfg)
	deps = deps.withDefaults()
	// The real driver constructor arms the dead-shell guard, so a smoke boot is rejected like a phase launch.
	rc, _ = d.Launch(ctx, cfg, deps)
	// runTmuxREPL's deferred cleanup writes the final scrollback on both the booted and timed-out paths.
	if b, err := os.ReadFile(filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")); err == nil {
		scrollback = string(b)
	}
	return rc, scrollback
}

// ScrollbackTail returns the last n non-empty lines of s, the boot pane tail `evolve doctor boot` and the loop readiness gate show.
func ScrollbackTail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	out := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			out = append([]string{lines[i]}, out...)
		}
	}
	return strings.Join(out, "\n")
}
