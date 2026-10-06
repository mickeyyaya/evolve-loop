package swarm

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func privateTmuxServer(t *testing.T) func(args ...string) error {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	dir, err := os.MkdirTemp("/tmp", "swx")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv(bridge.TmuxSocketEnv, "")
	tmux := func(args ...string) error {
		return exec.Command("tmux", bridge.TmuxSocketArgs(args...)...).Run()
	}
	t.Cleanup(func() {
		_ = tmux("kill-server")
		_ = os.RemoveAll(dir)
	})
	return tmux
}

func TestExecTmuxKill_NeverKillsASessionWhoseNameExtendsTheTarget(t *testing.T) {
	tmux := privateTmuxServer(t)
	const gone, longer = "evolve-bridge-r1-c5-build-pid42", "evolve-bridge-r1-c5-build-pid421"
	if err := tmux("-f", "/dev/null", "new-session", "-d", "-s", longer); err != nil {
		t.Fatalf("new-session %s: %v", longer, err)
	}

	if err := ExecTmuxKill(context.Background(), gone); err != nil {
		t.Fatalf("ExecTmuxKill(%q): %v", gone, err)
	}

	if err := tmux("has-session", "-t", bridge.ExactSessionTarget(longer)); err != nil {
		t.Fatalf("killing the missing session %q killed the live %q", gone, longer)
	}
}
