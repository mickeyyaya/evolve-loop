package bridge

// tmux_project_root_env_test.go — the pane shell carries the plane's root.
//
// Headless drivers hand the inner CLI driverEnv(deps) — the process env plus
// Deps.Env — but a tmux pane is a shell the bridge did not start: it inherits
// the tmux server's environment, and the bridge only ever sends it `cd
// <worktree>` and the launch command. So every `evolve` subcommand an agent
// runs in that pane resolves its root through cmdutil.EnvOrCwd → the cycle
// worktree — whose .evolve/inbox is a git-tracked snapshot of the plane's
// queue. Batch cycle 1631 (2026-09-12) claimed an inbox item in that copy;
// the plane's item never moved. core/phase.go documents ProjectRoot as
// "what a subprocess sees as EVOLVE_PROJECT_ROOT"; this pins that the pane
// sees it, in the one shell that survives into the agent's commands.

import (
	"strings"
	"testing"
)

func TestTmuxBoot_ExportsProjectRootIntoThePaneShell(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	root := t.TempDir()
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil, "--project-root="+root, "--worktree="+t.TempDir())

	cdAt, exportAt, launchAt := -1, -1, -1
	for i, k := range tmux.sentKeys {
		switch {
		case strings.HasPrefix(k, "cd "):
			cdAt = i
		case strings.HasPrefix(k, "export EVOLVE_PROJECT_ROOT="):
			exportAt = i
			if !strings.Contains(k, root) {
				t.Fatalf("export names %q, want the launch's project root %q", k, root)
			}
		case strings.Contains(k, "claude --model"):
			launchAt = i
		}
	}
	if exportAt < 0 {
		t.Fatalf("the pane shell never received `export EVOLVE_PROJECT_ROOT=...` — any `evolve` subcommand the agent runs resolves its root from the worktree cwd instead of the plane (cycle 1631's phantom claim); sent=%v", tmux.sentKeys)
	}
	if !(cdAt < exportAt && exportAt < launchAt) {
		t.Fatalf("export must follow the cd and precede the launch so the inner CLI inherits it (cd=%d export=%d launch=%d)", cdAt, exportAt, launchAt)
	}
}

// TestTmuxBoot_NoProjectRootMeansNoExport pins the guard: an empty root must
// not clobber a pane variable with an empty export (the fallback is cwd, by
// contract), which a benign "always export" refactor would do.
func TestTmuxBoot_NoProjectRootMeansNoExport(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil, "--worktree="+t.TempDir())
	for _, k := range tmux.sentKeys {
		if strings.HasPrefix(k, "export EVOLVE_PROJECT_ROOT=") {
			t.Fatalf("an empty project root produced an export: %q", k)
		}
	}
}
