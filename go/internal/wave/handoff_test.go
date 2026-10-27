package wave_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestReadCycles_TheFixabilitySignalsGiveTheHandoffFields(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-6/cycle-state.json", `{"cycle_id":6,"phase":"retrospective"}`)
	p.write(".evolve/runs/cycle-6/signals.ndjson",
		eventAt(1, "phase.outcome", "REPAIR_HANDOFF", `"decision":"would_stop","reason":"handoff_blocker","blockers":"M1"`)+
			eventAt(1, "phase.outcome", "REPAIR_HANDOFF", `"decision":"would_stop","reason":"handoff_blocker","blockers":"M1"`)+
			eventAt(2, "phase.outcome", "REPAIR_HANDOFF", `"decision":"none"`)+
			eventAt(3, "phase.outcome", "REPAIR_HANDOFF", `"decision":"stop","reason":"handoff_blocker","blockers":"M1"`))

	got, _ := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || got[0].HandoffWould != 2 || got[0].HandoffStop != "handoff_blocker: M1" {
		t.Errorf("ReadCycles = %+v, want 2 would stops and the stop reason", got)
	}
}
