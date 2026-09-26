package observer

import (
	"fmt"
	"testing"
)

func TestTmuxPaneProbe_NoMatchingSession(t *testing.T) {
	t.Parallel()
	run := func(args ...string) ([]byte, error) {
		if args[0] == "ls" {
			// A different cycle/phase + an unrelated session.
			return []byte("misc\nevolve-bridge-agy-c1-scout-pid2-9\n"), nil
		}
		return nil, fmt.Errorf("capture-pane should not be reached")
	}
	probe := newTmuxPaneProbe(190, "build", "", run)
	if probe() {
		t.Error("no session matching -c190-build- → want false")
	}
}

func TestTmuxPaneProbe_TmuxError(t *testing.T) {
	t.Parallel()
	run := func(args ...string) ([]byte, error) { return nil, fmt.Errorf("tmux: not found") }
	probe := newTmuxPaneProbe(190, "build", "", run)
	if probe() {
		t.Error("tmux error → want false")
	}
}

func TestTmuxPaneProbe_PaneAnimatingIsAlive(t *testing.T) {
	t.Parallel()
	pane := "Incubating… (12m 48s · ↑ 54.0k tokens)"
	run := func(args ...string) ([]byte, error) {
		switch args[0] {
		case "ls":
			return []byte("evolve-bridge-agy-c190-build-pid90464-1780402698\n"), nil
		case "capture-pane":
			return []byte(pane), nil
		}
		return nil, fmt.Errorf("unexpected args %v", args)
	}
	probe := newTmuxPaneProbe(190, "build", "", run)

	if !probe() {
		t.Fatal("first sighting of a live pane → want true")
	}
	pane = "Incubating… (12m 49s · ↑ 54.1k tokens)" // spinner advanced
	if !probe() {
		t.Error("pane changed between checks → want true (agent alive mid-turn)")
	}
	if probe() {
		t.Error("pane frozen between checks → want false (no liveness claim)")
	}
}

func TestTmuxPaneProbe_CaptureError(t *testing.T) {
	t.Parallel()
	run := func(args ...string) ([]byte, error) {
		switch args[0] {
		case "ls":
			return []byte("evolve-bridge-claude-c5-audit-pid1-2\n"), nil
		case "capture-pane":
			return nil, fmt.Errorf("can't find pane")
		}
		return nil, fmt.Errorf("unexpected %v", args)
	}
	probe := newTmuxPaneProbe(5, "audit", "", run)
	if probe() {
		t.Error("capture-pane error → want false")
	}
}
