package bridge

import (
	"strings"
	"testing"
)

func TestTmuxBoot_DetachesThePaneShellFromTheRunServerBeforeTheLaunch(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil, "--project-root="+t.TempDir(), "--worktree="+t.TempDir())

	cdAt, unsetAt, launchAt, unsets := -1, -1, -1, 0
	for i, k := range tmux.sentKeys {
		switch {
		case strings.HasPrefix(k, "cd "):
			cdAt = i
		case k == "unset TMUX TMUX_PANE":
			unsetAt = i
			unsets++
		case strings.Contains(k, "claude --model"):
			launchAt = i
		}
	}
	if unsets != 1 {
		t.Fatalf("the pane shell must receive `unset TMUX TMUX_PANE` exactly once, got %d; keys sent: %q", unsets, tmux.sentKeys)
	}
	if !(cdAt < unsetAt && unsetAt < launchAt) {
		t.Fatalf("the unset must follow the cd and precede the CLI launch, so a tmux client the agent starts cannot reach the run server (cd=%d unset=%d launch=%d); keys sent: %q", cdAt, unsetAt, launchAt, tmux.sentKeys)
	}
}
