package observer

import (
	"fmt"
	"testing"
)

// TestTmuxPaneProbe_NoMatchingSession — when no evolve-bridge session matches
// the cycle/phase infix, the probe makes no liveness claim (false), so a
// non-tmux phase never has its stall masked.
func TestTmuxPaneProbe_NoMatchingSession(t *testing.T) {
	run := func(args ...string) ([]byte, error) {
		if args[0] == "ls" {
			// A different cycle/phase + an unrelated session.
			return []byte("misc\nevolve-bridge-agy-c1-scout-pid2-9\n"), nil
		}
		return nil, fmt.Errorf("capture-pane should not be reached")
	}
	probe := newTmuxPaneProbe(190, "build", run)
	if probe() {
		t.Error("no session matching -c190-build- → want false")
	}
}

// TestTmuxPaneProbe_TmuxError — tmux absent / ls error → false (no claim).
func TestTmuxPaneProbe_TmuxError(t *testing.T) {
	run := func(args ...string) ([]byte, error) { return nil, fmt.Errorf("tmux: not found") }
	probe := newTmuxPaneProbe(190, "build", run)
	if probe() {
		t.Error("tmux error → want false")
	}
}

// TestTmuxPaneProbe_PaneAnimatingIsAlive — the cycle-190 case: a matching
// session whose pane content changes between checks (spinner / token counter
// advancing) reports alive. The first sighting also reports alive (grace
// window); an unchanged pane afterward reports not-alive (possibly hung).
func TestTmuxPaneProbe_PaneAnimatingIsAlive(t *testing.T) {
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
	probe := newTmuxPaneProbe(190, "build", run)

	if !probe() {
		t.Fatal("first sighting of a live pane → want true")
	}
	pane = "Incubating… (12m 49s · ↑ 54.1k tokens)" // spinner advanced
	if !probe() {
		t.Error("pane changed between checks → want true (agent alive mid-turn)")
	}
	// pane unchanged from the previous check now:
	if probe() {
		t.Error("pane frozen between checks → want false (no liveness claim)")
	}
}

// TestTmuxPaneProbe_CaptureError — session found but capture-pane errors
// (pane died mid-check) → false.
func TestTmuxPaneProbe_CaptureError(t *testing.T) {
	run := func(args ...string) ([]byte, error) {
		switch args[0] {
		case "ls":
			return []byte("evolve-bridge-claude-c5-audit-pid1-2\n"), nil
		case "capture-pane":
			return nil, fmt.Errorf("can't find pane")
		}
		return nil, fmt.Errorf("unexpected %v", args)
	}
	probe := newTmuxPaneProbe(5, "audit", run)
	if probe() {
		t.Error("capture-pane error → want false")
	}
}
