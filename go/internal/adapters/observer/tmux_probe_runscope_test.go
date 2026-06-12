// tmux_probe_runscope_test.go — CB.6 contract (concurrency campaign W4):
// the observer's pane-liveness probe asserts RUN ownership before making a
// liveness claim. The probe's match grants stall-clock extensions; matching
// ANOTHER run's session would keep a dead agent's clock fresh forever (the
// cross-run variant of the cycles-254/255 false-liveness class). Fail-closed:
// a probe that knows its run id refuses any session without the run token.
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
	// Another run's session matches cycle+phase infix but carries run token rBBBB1111.
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
	// RunID unknown (legacy single-driver dispatch) → infix-only match, the
	// pre-CB.6 behavior, byte-identical.
	run := fakeSessions("evolve-bridge-c190-build-pid9-7")
	probe := newTmuxPaneProbe(190, "build", "", run)
	if !probe() {
		t.Error("legacy probe (no run id) must keep matching un-scoped sessions")
	}
}
