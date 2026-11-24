package orchestrator

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (f *fixture) ejectionOf(cycle int, cause string, shadow bool) signalcenter.Event {
	f.t.Helper()
	e := &integration.Ejection{Cycle: cycle, Cause: cause, Diff: "d"}
	path, err := e.WriteBrief(f.t.TempDir())
	if err != nil {
		f.t.Fatal(err)
	}
	fields := e.Fields(path)
	if shadow {
		fields["shadow"] = "true"
	}
	return ejectedWith(cycle, fields)
}

func mergedEvent(cycle int) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleShip, Kind: signalcenter.KindShipMerged, Code: integration.CodeMerged}
}

func TestOversight_AConflictEjectionHalvesTheWindowAndAGateEjectionDoesNot(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.briefs(nil)
	f.addLane("A", 41)
	f.addLane("B", 42)
	f.start()

	f.send("ship", f.ejectionOf(41, integration.CauseConflict, false))
	f.send("ship", f.ejectionOf(42, integration.CauseGate, false))

	if clean, conflict := f.landing.counts(); clean != 0 || conflict != 1 {
		t.Fatalf("landing counts = %d clean, %d conflict, want 0 and 1", clean, conflict)
	}
	got := f.actions()
	if len(got) != 2 || got[0].Fields["actions"] != "count_conflict,brief_lane,hold_cells" || got[1].Fields["actions"] != "brief_lane,hold_cells" {
		t.Fatalf("action records = %+v", got)
	}
}

func TestOversight_AShadowEjectionDoesNotHalveTheWindow(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.briefs(nil)
	f.addLane("A", 41)
	f.start()

	f.send("ship", f.ejectionOf(41, integration.CauseConflict, true))

	if _, conflict := f.landing.counts(); conflict != 0 {
		t.Fatalf("conflicts = %d, want 0 for an ejection stamped shadow", conflict)
	}
}

func TestOversight_AnEjectionAtOrchestratorShadowOnlyRecords(t *testing.T) {
	f := newFixture(t, StageShadow)
	f.briefs(nil)
	f.addLane("A", 41)
	f.start()

	f.send("ship", f.ejectionOf(41, integration.CauseConflict, false))

	got := f.actions()
	if _, conflict := f.landing.counts(); conflict != 0 || len(got) != 1 || got[0].Fields["applied"] != "false" {
		t.Fatalf("conflicts = %d, actions = %+v, want a record that applied nothing", conflict, got)
	}
}

func TestOversight_AConflictEjectionOfACycleWithNoKnownLaneStillHalvesTheWindow(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.briefs(nil)
	f.addLane("A", 41)
	f.start()

	f.send("ship", f.ejectionOf(99, integration.CauseConflict, false))

	got := f.actions()
	if _, conflict := f.landing.counts(); conflict != 1 || len(got) != 1 || got[0].Fields["actions"] != "count_conflict" {
		t.Fatalf("conflicts = %d, actions = %+v, want one count_conflict", conflict, got)
	}
}

func TestOversight_AConflictEjectionAfterTheLandingIsIgnored(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", landedEvent(41))

	f.send("ship", f.ejectionOf(41, integration.CauseConflict, false))

	if _, conflict := f.landing.counts(); conflict != 0 {
		t.Fatalf("conflicts = %d, want 0 after the cycle landed", conflict)
	}
}

func TestOversight_AMergedMemberFreesTheLaneAndGrowsTheWindow(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.briefs(nil)
	f.addLane("A", 41)
	f.intake.Hold("A")
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	f.setCycle("A", 0)

	f.send("ship", mergedEvent(41))

	if clean, conflict := f.landing.counts(); clean != 1 || conflict != 0 {
		t.Fatalf("landing counts = %d clean, %d conflict, want 1 and 0", clean, conflict)
	}
	got := f.actions()
	last := got[len(got)-1]
	if last.Fields["actions"] != "free_cells,count_clean_landing,backfill" || last.Fields["lane"] != "A" {
		t.Fatalf("action records = %+v, want free_cells,count_clean_landing,backfill on lane A", got)
	}
	if len(f.warnsOfCycle(41)) != 0 {
		t.Fatalf("a merged member of a delivered lane was signalled unlocated: %+v", f.warnsOfCycle(41))
	}
}

func TestOversight_AMergedMemberWhoseLaneLeftIsStillCounted(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	f.endLane("A")

	f.send("ship", mergedEvent(41))

	if clean, _ := f.landing.counts(); clean != 1 {
		t.Fatalf("clean = %d, want 1", clean)
	}
	got := f.actions()
	if got[len(got)-1].Fields["actions"] != "count_clean_landing" {
		t.Fatalf("action records = %+v, want count_clean_landing", got)
	}
}

func TestOversight_AMergedMemberOfAnUnknownCycleIsCountedWithNoLane(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	f.send("ship", mergedEvent(99))

	if clean, _ := f.landing.counts(); clean != 1 {
		t.Fatalf("clean = %d, want 1", clean)
	}
}

func TestOversight_ASecondMergedForTheSameCycleCountsOnce(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))

	f.send("ship", mergedEvent(41))
	f.send("ship", mergedEvent(41))

	if clean, _ := f.landing.counts(); clean != 1 {
		t.Fatalf("clean = %d, want 1", clean)
	}
	if got := f.actions(); !reflect.DeepEqual(len(got), 2) {
		t.Fatalf("action records = %+v, want the seal and one merge", got)
	}
}

func TestOversight_AMergedMemberWithNoWindowObserverIsStillHandled(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.over.Landing = nil
	f.addLane("A", 41)
	f.start()

	f.send("ship", mergedEvent(41))

	if got := f.actions(); len(got) != 1 {
		t.Fatalf("action records = %+v, want one", got)
	}
}

func TestSubscribe_TheShipChannelRouteCarriesAShipMergedEvent(t *testing.T) {
	route, _, err := filter.Parse(channelRoutes["ship"], filter.RegisteredCatalog())
	if err != nil {
		t.Fatal(err)
	}
	merged := mergedEvent(41)
	if !route.Match(filter.Record{Signal: &merged}) {
		t.Fatalf("route %q does not match a ship.merged event", channelRoutes["ship"])
	}
}
