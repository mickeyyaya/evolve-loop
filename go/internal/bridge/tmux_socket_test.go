package bridge

import "testing"

// TestTmuxSocket_IsDedicatedNotDefault: the bridge must run its agent panes on a
// dedicated tmux socket, never the user's shared default. On the default socket a
// stray `tmux attach` lands in a live agent REPL and the operator's keystrokes go
// to it (the flag-campaign-8 "show progress" leak).
func TestTmuxSocket_IsDedicatedNotDefault(t *testing.T) {
	if TmuxSocket == "" || TmuxSocket == "default" {
		t.Fatalf("TmuxSocket = %q; must be a dedicated, non-default socket name", TmuxSocket)
	}
}

// TestTmuxSocketArgs_PrependsGlobalSocketSelector: -L is a GLOBAL tmux flag and
// must precede the subcommand, so a wrapped invocation targets the isolated
// server. It is the single SSOT every bridge tmux consumer (execTmux, swarm
// teardown, observer probe) routes through.
func TestTmuxSocketArgs_PrependsGlobalSocketSelector(t *testing.T) {
	got := TmuxSocketArgs("capture-pane", "-t", "sess", "-p")
	want := []string{"-L", TmuxSocket, "capture-pane", "-t", "sess", "-p"}
	if len(got) != len(want) {
		t.Fatalf("TmuxSocketArgs len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("TmuxSocketArgs[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestTmuxSocketArgs_EmptyStillSelectsSocket: even a bare call carries the
// selector, so a consumer can never accidentally hit the default socket.
func TestTmuxSocketArgs_EmptyStillSelectsSocket(t *testing.T) {
	got := TmuxSocketArgs()
	if len(got) != 2 || got[0] != "-L" || got[1] != TmuxSocket {
		t.Fatalf("TmuxSocketArgs() = %v, want [-L %s]", got, TmuxSocket)
	}
}
