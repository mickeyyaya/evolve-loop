package faketmux

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func awaitCommand(t *testing.T, f fakeTmux, session, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for got := ""; got != want+"\n"; got = f.must("display-message", "-p", "-t", "="+session+":", "#{pane_current_command}") {
		if time.Now().After(deadline) {
			t.Fatalf("the pane never showed %q as its foreground program (last %q)", want, got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestFakeTmux_PaneLeaderAndForegroundCommandRunWithoutPS(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	f := newFakeTmux(t)
	f.must("new-session", "-d", "-s", "np", "/bin/sleep", "30")
	pid, err := strconv.Atoi(strings.TrimSpace(f.must("list-panes", "-s", "-t", "=np:", "-F", "#{pane_pid}")))
	if err != nil {
		t.Fatal(err)
	}

	awaitCommand(t, f, "np", "sleep")
	if !isPaneLeader(pid) {
		t.Errorf("pane %d is not seen as the leader of its group: the shell name must come with no ps binary", pid)
	}
	f.must("kill-session", "-t", "=np:")

	awaitGone(t, pid)
}
