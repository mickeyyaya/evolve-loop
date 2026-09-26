package swarm

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestExecTmuxKill_TargetsIsolatedBridgeSocket(t *testing.T) {
	// Clear the per-run socket so the test pins the default socket name whatever the shell sets.
	t.Setenv(bridge.TmuxSocketEnv, "")
	var got []string
	orig := tmuxRun
	tmuxRun = func(_ context.Context, args ...string) error { got = args; return nil }
	defer func() { tmuxRun = orig }()

	const sess = "evolve-bridge-r1-c5-build-pid9-ts"
	if err := ExecTmuxKill(context.Background(), sess); err != nil {
		t.Fatalf("ExecTmuxKill: %v", err)
	}

	want := []string{"-L", bridge.TmuxSocket, "kill-session", "-t", sess}
	if len(got) != len(want) {
		t.Fatalf("tmux args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tmux args[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
