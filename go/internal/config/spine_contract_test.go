package config_test

// External test (config_test package) so it can import core. Pins the
// config-local staticSpinePhases set against the actual state machine — if
// someone adds a new built-in phase to core but forgets to extend the spine
// set, this fails loudly. Encoded here rather than inside config because the
// production config package must not import core (leaf invariant).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestStaticSpineMatchesStateMachine is the cross-package contract: every
// runnable built-in phase the state machine drives must appear in
// config.StaticSpinePhases (exported via the test seam below), and vice versa.
// Sentinel phases (start, end) are excluded — they're not agent runs.
func TestStaticSpineMatchesStateMachine(t *testing.T) {
	want := map[string]struct{}{
		string(core.PhaseIntent):       {},
		string(core.PhaseScout):        {},
		string(core.PhaseTriage):       {},
		string(core.PhaseTDD):          {},
		string(core.PhaseBuildPlanner): {},
		string(core.PhaseBuild):        {},
		string(core.PhaseAudit):        {},
		string(core.PhaseShip):         {},
		string(core.PhaseRetro):        {},
	}
	got := config.StaticSpinePhasesForTesting()
	for p := range want {
		if _, ok := got[p]; !ok {
			t.Errorf("state machine knows phase %q but config.staticSpinePhases is missing it", p)
		}
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("config.staticSpinePhases lists %q but the state machine has no such built-in phase", p)
		}
	}
}
