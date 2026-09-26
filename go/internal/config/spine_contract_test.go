package config_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestStaticSpineMatchesStateMachine(t *testing.T) {
	// start and end are sentinels, not agent runs, so want omits them.
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
