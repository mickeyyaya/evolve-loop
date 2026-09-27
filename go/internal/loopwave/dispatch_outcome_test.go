package loopwave

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

type launchesNothing struct{ calls int }

func (l *launchesNothing) Run(context.Context, []fleet.CycleSpec) []fleet.Result {
	l.calls++
	return nil
}

func TestDispatch_AWaveThatLaunchedNoLaneDidNotRun(t *testing.T) {
	h := newHarness(t)
	gate := &launchesNothing{}
	out, err := h.e.Dispatch(context.Background(), request(2, waveFC(2), pass, twoCardPlan, gate))
	if err != nil || gate.calls != 1 || out.Ran || out.Specs != nil || out.Results != nil {
		t.Errorf("the gate launched nothing, so the wave did not run: %+v %v calls=%d", out, err, gate.calls)
	}
	repair, err := h.e.ForceOneLane(context.Background(), request(1, waveFC(1), pass, twoCardPlan, gate))
	if err != nil || gate.calls != 2 || repair.Ran {
		t.Errorf("the one-lane repair reports the same truth: %+v %v calls=%d", repair, err, gate.calls)
	}
}

func TestForceOneLane_LaunchesAtMostTheRepairWidth(t *testing.T) {
	h := newHarness(t)
	out, err := h.e.ForceOneLane(context.Background(), request(1, waveFC(2), pass, twoCardPlan, &recorder{}))
	if err != nil || len(out.Specs) != RepairWidth || len(out.Results) != RepairWidth {
		t.Errorf("the repair launches RepairWidth lanes from a two-lane plan: %+v %v", out, err)
	}
}
