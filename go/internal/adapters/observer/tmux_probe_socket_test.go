package observer

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestSocketTmuxRunner_PrependsIsolatedSocket(t *testing.T) {
	// TmuxSocketArgs reads a per-run socket from the env; clearing it pins the default name.
	t.Setenv(bridge.TmuxSocketEnv, "")
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
