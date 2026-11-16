package orchestrator

import (
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const unlocatedCycle = 99

func (f *fixture) warnsOfCycle(cycle int) []signalcenter.Event {
	f.t.Helper()
	f.events.mu.Lock()
	defer f.events.mu.Unlock()
	var out []signalcenter.Event
	for _, e := range f.events.events {
		if e.Severity == signalcenter.SeverityWarn && e.Fields["cycle"] == strconv.Itoa(cycle) {
			out = append(out, e)
		}
	}
	return out
}

func TestOversight_AnUnlocatedLandingOrSealEmitsOneWarnSignalNamingItsCycleAndEvent(t *testing.T) {
	for name, tc := range map[string]struct {
		event signalcenter.Event
		kind  signalcenter.Kind
	}{
		"a landing": {event: landedEvent(unlocatedCycle), kind: signalcenter.KindShipLanded},
		"a seal":    {event: sealedEvent(unlocatedCycle, cyclestate.VerdictPASS), kind: signalcenter.KindCycleSealed},
	} {
		for _, stage := range []string{StageShadow, StageEnforce} {
			f := newFixture(t, stage)
			f.addLane("A", 41)
			f.start()

			f.send("cycle", tc.event)

			got := f.warnsOfCycle(unlocatedCycle)
			if len(got) != 1 {
				t.Errorf("%s at stage %s: WARN signals naming cycle %d = %+v, want exactly one: no live lane holds it", name, stage, unlocatedCycle, got)
				continue
			}
			if got[0].Fields["event"] != string(tc.kind) {
				t.Errorf("%s at stage %s: signal event = %q, want %q", name, stage, got[0].Fields["event"], tc.kind)
			}
			if _, registered := signalcenter.IsRegistered(got[0].Code); !registered {
				t.Errorf("%s at stage %s: signal code %q is not registered", name, stage, got[0].Code)
			}
		}
	}
}

func TestOversight_AnUnlocatedLandingKeepsTheLandingStateCleanup(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	f.send("cycle", landedEvent(unlocatedCycle))

	if !f.over.ended[unlocatedCycle] {
		t.Error("an unlocated landing did not mark its cycle ended: the signal must not replace the existing cleanup")
	}
	if clean, conflict := f.landing.counts(); clean != 0 || conflict != 0 {
		t.Errorf("landing counts = %d clean, %d conflict, want 0 and 0: a landing no lane holds is not counted", clean, conflict)
	}
}

func TestOversight_AnUnlocatedLandingSignalIsAbsentForALocatedLane(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))

	f.send("cycle", landedEvent(41))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))

	if got := f.warnsOfCycle(41); len(got) != 0 {
		t.Fatalf("WARN signals naming cycle 41 = %+v, want none: lane A holds it", got)
	}
	if got := f.actions(); len(got) != 2 {
		t.Errorf("action records = %+v, want the landing and the seal recorded as before", got)
	}
}
