package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// BootSmokeTest performs a REAL boot of the given *-tmux driver — NewSession,
// cd, optional sandbox prefix, launch the CLI, poll for the prompt marker — then
// cleanly exits the REPL WITHOUT delivering a prompt or waiting for an artifact
// (it sets cfg.BootOnly and routes through the driver's normal Launch, so the
// boot it exercises is byte-identical to a real phase launch). This is the
// readiness check that catches an ExitREPLBootTimeout (exit 80) BEFORE a cycle
// commits any LLM budget.
//
// Returns a bridge exit code and the captured pane scrollback (for diagnosis on
// failure):
//   - ExitOK              — REPL booted to its prompt marker
//   - ExitREPLBootTimeout — marker never appeared within the boot deadline
//   - ExitMissingBinary   — the CLI binary is absent
//   - ExitBadFlags        — driverName is unknown or not a *-tmux driver
//
// The caller supplies cfg (Worktree set + Agent="build" exercises the sandboxed
// write-phase boot path) and deps (real seams by default; tests inject fakeTmux).
func BootSmokeTest(ctx context.Context, driverName string, cfg *Config, deps Deps) (rc int, scrollback string) {
	d, ok := LookupDriver(driverName)
	if !ok || !strings.HasSuffix(driverName, "-tmux") {
		// Only the interactive *-tmux drivers have a bootable REPL to smoke-test.
		return ExitBadFlags, ""
	}
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.CLI = driverName
	cfg.BootOnly = true
	cfg.AllowBypass = true // boot-only runs no task; bypass-equivalent so the safety gate passes
	if cfg.Workspace == "" {
		// Self-provision a workspace for callers passing a minimal cfg, and own
		// its lifecycle: the dir is written to (tmux-final-scrollback.txt) and
		// must not leak. The deferred cleanup fires AFTER the scrollback read
		// below, so the returned scrollback string is unaffected.
		tmp, err := os.MkdirTemp("", "evolve-bootsmoke-*")
		if err != nil {
			return ExitBadFlags, ""
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cfg.Workspace = tmp
	}
	deps = deps.withDefaults()
	rc, _ = d.Launch(ctx, cfg, deps)
	// runTmuxREPL's deferred tmuxCleanup writes the final scrollback here on both
	// the booted and timed-out paths — read it back for the caller's diagnostic.
	if b, err := os.ReadFile(filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")); err == nil {
		scrollback = string(b)
	}
	return rc, scrollback
}
