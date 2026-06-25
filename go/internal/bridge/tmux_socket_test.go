package bridge

import (
	"strings"
	"testing"
)

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

// TestTmuxSocketArgs_PerRunOverride (F6): TmuxSocketEnv selects a per-run socket
// at call time so concurrent runs target distinct tmux servers; unset ⇒ default.
func TestTmuxSocketArgs_PerRunOverride(t *testing.T) {
	t.Run("default when unset", func(t *testing.T) {
		t.Setenv(TmuxSocketEnv, "")
		got := TmuxSocketArgs("ls")
		if len(got) != 3 || got[1] != TmuxSocket {
			t.Fatalf("got %v, want [-L %s ls]", got, TmuxSocket)
		}
	})
	t.Run("per-run override", func(t *testing.T) {
		t.Setenv(TmuxSocketEnv, "evolve-bridge-p999")
		got := TmuxSocketArgs("kill-session", "-t", "x")
		if got[0] != "-L" || got[1] != "evolve-bridge-p999" || got[len(got)-1] != "x" {
			t.Fatalf("got %v, want -L evolve-bridge-p999 … x", got)
		}
	})
}

// TestDeriveRunSocket (F6): the per-run socket extends the base name with the
// loop pid, yielding a valid tmux -L name the orphan-socket GC can parse back.
func TestDeriveRunSocket(t *testing.T) {
	if s := DeriveRunSocket(12345); s != "evolve-bridge-p12345" {
		t.Fatalf("DeriveRunSocket(12345) = %q, want evolve-bridge-p12345", s)
	}
	if !strings.HasPrefix(DeriveRunSocket(1), TmuxSocket+"-") {
		t.Fatalf("derived socket must extend the base %q", TmuxSocket)
	}
}
