package swarm

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// TestExecTmuxKill_TargetsIsolatedBridgeSocket: teardown must kill sessions on
// the bridge's dedicated socket, not the default. If the driver moved agent panes
// to the isolated socket (bridge.TmuxSocket) but the killer stayed on the default,
// reaped sessions would be invisible to it and leak as orphans.
func TestExecTmuxKill_TargetsIsolatedBridgeSocket(t *testing.T) {
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
