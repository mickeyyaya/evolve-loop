package observer

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge"
)

// TestSocketTmuxRunner_PrependsIsolatedSocket: the observer's liveness probe must
// query the bridge's dedicated tmux socket, not the operator's default. If the
// driver moved agent panes to bridge.TmuxSocket but the probe stayed on the
// default, `tmux ls` would never see them — the probe would report every agent
// dead and the stall clock would never be reset by genuine liveness.
func TestSocketTmuxRunner_PrependsIsolatedSocket(t *testing.T) {
	var got []string
	wrapped := socketTmuxRunner(func(args ...string) ([]byte, error) {
		got = args
		return []byte("ok"), nil
	})

	if _, err := wrapped("ls", "-F", "#{session_name}"); err != nil {
		t.Fatalf("wrapped runner: %v", err)
	}
	want := []string{"-L", bridge.TmuxSocket, "ls", "-F", "#{session_name}"}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
