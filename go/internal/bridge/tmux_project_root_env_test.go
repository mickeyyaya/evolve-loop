package bridge

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
