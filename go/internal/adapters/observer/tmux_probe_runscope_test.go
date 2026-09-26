package observer

import (
	"fmt"
	"testing"
)

// fakeSessions builds a tmuxRunner serving `tmux ls` and capture-pane.
func fakeSessions(names ...string) tmuxRunner {
	calls := 0
	return func(args ...string) ([]byte, error) {
		if args[0] == "ls" {
			out := ""
			for _, n := range names {
				out += n + "\n"
			}
			return []byte(out), nil
		}
		calls++
		return []byte(fmt.Sprintf("pane frame %d", calls)), nil
	}
}

func TestProbeRefusesForeignRunSession(t *testing.T) {
	t.Parallel()
	// The cycle and phase infix match, but the run token belongs to another run.
	run := fakeSessions("evolve-bridge-rBBBB1111-c190-build-pid9-7")
	probe := newTmuxPaneProbe(190, "build", "01AAAA22XXXXXXXXXXXXXXXXXX", run)
	if probe() {
		t.Error("probe claimed liveness from ANOTHER run's session — run-scope assertion must fail closed")
	}
}

func TestProbeMatchesOwnRunSession(t *testing.T) {
	t.Parallel()
	run := fakeSessions("evolve-bridge-r01AAAA22-c190-build-pid9-7")
	probe := newTmuxPaneProbe(190, "build", "01AAAA22XXXXXXXXXXXXXXXXXX", run)
	if !probe() {
		t.Error("probe refused its OWN run's session (first sighting must grant a window)")
	}
}

func TestProbeLegacyNoRunIDMatchesAny(t *testing.T) {
	t.Parallel()
	run := fakeSessions("evolve-bridge-c190-build-pid9-7")
	probe := newTmuxPaneProbe(190, "build", "", run)
	if !probe() {
		t.Error("legacy probe (no run id) must keep matching un-scoped sessions")
	}
}
