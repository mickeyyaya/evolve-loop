package bridge

import (
	"strings"
	"testing"
)

func TestTmuxSocket_IsDedicatedNotDefault(t *testing.T) {
	if TmuxSocket == "" || TmuxSocket == "default" {
		t.Fatalf("TmuxSocket = %q; must be a dedicated, non-default socket name", TmuxSocket)
	}
}

func TestTmuxSocketArgs_PrependsGlobalSocketSelector(t *testing.T) {
	t.Setenv(TmuxSocketEnv, "")
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

func TestTmuxSocketArgs_EmptyStillSelectsSocket(t *testing.T) {
	t.Setenv(TmuxSocketEnv, "")
	got := TmuxSocketArgs()
	if len(got) != 2 || got[0] != "-L" || got[1] != TmuxSocket {
		t.Fatalf("TmuxSocketArgs() = %v, want [-L %s]", got, TmuxSocket)
	}
}

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

func TestDeriveRunSocket(t *testing.T) {
	if s := DeriveRunSocket(12345); s != "evolve-bridge-p12345" {
		t.Fatalf("DeriveRunSocket(12345) = %q, want evolve-bridge-p12345", s)
	}
	if !strings.HasPrefix(DeriveRunSocket(1), TmuxSocket+"-") {
		t.Fatalf("derived socket must extend the base %q", TmuxSocket)
	}
}
