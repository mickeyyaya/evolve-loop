package orchestrator

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type briefLog struct {
	mu    sync.Mutex
	sent  []string
	fail  error
	lanes []string
}

func (b *briefLog) send(lane, text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lanes = append(b.lanes, lane)
	b.sent = append(b.sent, text)
	return b.fail
}

func (b *briefLog) snapshot() (lanes, texts []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.lanes...), append([]string(nil), b.sent...)
}

func (f *fixture) briefs(fail error) *briefLog {
	b := &briefLog{fail: fail}
	f.over.Brief = b.send
	return b
}

func (f *fixture) ejection(cycle int, text string) signalcenter.Event {
	f.t.Helper()
	e := &integration.Ejection{Cycle: cycle, Cause: integration.CauseConflict, Diff: text}
	path, err := e.WriteBrief(f.t.TempDir())
	if err != nil {
		f.t.Fatal(err)
	}
	return ejectedWith(cycle, e.Fields(path))
}

func ejectedWith(cycle int, fields map[string]string) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleShip, Kind: signalcenter.KindShipError,
		Severity: signalcenter.SeverityWarn, Code: CodeIntegrationEjected, Fields: fields}
}

func briefOf(cycle int, text string) string {
	return (&integration.Ejection{Cycle: cycle, Cause: integration.CauseConflict, Diff: text}).Brief()
}

func TestOversight_AnEjectionSendsTheBriefToItsLaneAndKeepsItsCellsHeldUntilTheLandingFreesThem(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.intake.Free("A")
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}

	f.send("ship", f.ejection(41, "the change"))

	if lanes, texts := b.snapshot(); !reflect.DeepEqual(lanes, []string{"A"}) || !reflect.DeepEqual(texts, []string{briefOf(41, "the change")}) {
		t.Fatalf("briefs = %v %v, want the brief file of the event sent to lane A once", lanes, texts)
	}
	if f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Fatal("after the ejection, the cells of A are free, want them held again")
	}
	if got := f.actions(); len(got) != 1 || got[0].Fields["actions"] != "brief_lane,hold_cells" || got[0].Fields["lane"] != "A" || got[0].Fields["applied"] != "true" {
		t.Fatalf("action records = %+v, want brief_lane,hold_cells on A applied", got)
	}
	f.send("ship", landedEvent(41))
	if !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("after the landing, the cells of A are still held, want them free")
	}
}

func TestOversight_AnEjectedLaneThatGivesUpFreesItsCells(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}

	f.send("ship", f.ejection(41, "d"))
	if f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Fatal("after the ejection, the cells of A are free, want them held")
	}
	f.send("cycle", sealedEvent(41, cyclestate.VerdictFAIL))

	if !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("after the FAIL seal, the cells of A are still held, want them free")
	}
}

func TestOversight_AShadowEjectionRecordsTheBriefAndSendsAndHoldsNothing(t *testing.T) {
	f := newFixture(t, StageShadow)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.intake.Free("A")
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 {
		t.Errorf("the shadow sent briefs to %v, want none", lanes)
	}
	if !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("the shadow held the cells of A, want them free")
	}
	if got := f.actions(); len(got) != 1 || got[0].Fields["actions"] != "brief_lane,hold_cells" || got[0].Fields["applied"] != "false" {
		t.Errorf("action records = %+v, want brief_lane,hold_cells with applied false", got)
	}
}

func TestOversight_AFailedBriefSendPublishesTheCodeAndKeepsTheHold(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.briefs(errors.New("no pane"))
	f.addLane("A", 41)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}

	f.send("ship", f.ejection(41, "d"))

	failed := f.events.withCode(CodeEjectionBriefFailed)
	if len(failed) != 1 || failed[0].Severity != signalcenter.SeverityWarn || failed[0].Fields["lane"] != "A" || !strings.Contains(failed[0].Fields["error"], "no pane") {
		t.Fatalf("failed events = %+v, want one WARN for lane A with the error", failed)
	}
	if got := failed[0].Fields; got["cycle"] != "41" || got["stage"] != StageEnforce || got["applied"] != "true" {
		t.Errorf("failed fields = %v, want cycle 41 and the stamp of the enforce stage", got)
	}
	if f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("a failed send freed the cells of A, want the hold kept")
	}
}

func TestOversight_AnEjectionWithNoBriefPortPublishesTheFailureAndKeepsTheHold(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}

	f.send("ship", f.ejection(41, "d"))

	if got := f.events.withCode(CodeEjectionBriefFailed); len(got) != 1 {
		t.Fatalf("failed events = %d, want 1: a missing port is a loud failure", len(got))
	}
	if f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("a missing port freed the cells of A, want the hold kept")
	}
}

func TestOversight_AnEjectionWhoseBriefFileIsMissingOrEmptyPublishesTheFailureAndSendsNothing(t *testing.T) {
	for name, fields := range map[string]map[string]string{
		"no field":    {},
		"not a brief": {integration.FieldBriefFile: "/etc/passwd"},
		"missing":     {integration.FieldBriefFile: integration.BriefPath(t.TempDir(), 41)},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, StageEnforce)
			b := f.briefs(nil)
			f.addLane("A", 41)
			f.start()
			inFlight := []fleet.Todo{todoIn("A", "inboxrank")}

			f.send("ship", ejectedWith(41, fields))

			if lanes, _ := b.snapshot(); len(lanes) != 0 {
				t.Errorf("a brief that cannot be read was sent to %v", lanes)
			}
			if got := f.events.withCode(CodeEjectionBriefFailed); len(got) != 1 {
				t.Errorf("failed events = %d, want 1", len(got))
			}
			if f.admits(inFlight, todoIn("d", "inboxrank")) {
				t.Error("an unreadable brief freed the cells of A, want the hold kept")
			}
		})
	}
}

func TestOversight_AnEjectionWithAnEmptyBriefFilePublishesTheFailure(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	path := integration.BriefPath(t.TempDir(), 41)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	f.send("ship", ejectedWith(41, map[string]string{integration.FieldBriefFile: path}))

	if lanes, _ := b.snapshot(); len(lanes) != 0 || len(f.events.withCode(CodeEjectionBriefFailed)) != 1 {
		t.Errorf("an empty brief reached %v, want one failure and no send", lanes)
	}
}

func TestOversight_AnEjectionOfALaneThatLeftRecordsLaneLeftAndChangesNothing(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.endLane("A")

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 {
		t.Errorf("a lane that left got briefs %v, want none", lanes)
	}
	got := f.actions()
	if len(got) != 1 || got[0].Fields["actions"] != "lane_left" || got[0].Fields["lane"] != "A" {
		t.Errorf("action records = %+v, want lane_left on A", got)
	}
}

func TestOversight_AnEjectionOfAnUnknownCycleIsIgnored(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()

	f.send("ship", f.ejection(99, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 || len(f.actions()) != 0 {
		t.Errorf("an unknown cycle got briefs %v and actions %+v, want none", lanes, f.actions())
	}
}

func TestOversight_AnEjectionAfterThePASSSealOfItsCycleStillBriefsTheLiveLane(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); !reflect.DeepEqual(lanes, []string{"A"}) {
		t.Errorf("briefs went to %v, want lane A: the seal came first, as it does when the ship stops at delivery", lanes)
	}
}

func TestOversight_AnEjectionAfterThePASSSealAndTheExitOfItsLaneRecordsLaneLeft(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	f.endLane("A")

	f.send("ship", f.ejection(41, "d"))

	got := f.actions()
	if lanes, _ := b.snapshot(); len(lanes) != 0 || got[len(got)-1].Fields["actions"] != "lane_left" {
		t.Errorf("briefs %v, actions %+v, want lane_left as the last record and no brief", lanes, got)
	}
}

func TestOversight_AnEjectionAfterAFAILSealHoldsNothingAndSendsNothing(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictFAIL))

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 || !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Errorf("a late ejection of a cycle that gave up sent %v or held the cells again", lanes)
	}
}

func TestOversight_ALandingEndsTheEjectionRightOfItsCycle(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	f.send("ship", landedEvent(41))

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 {
		t.Errorf("an ejection of a landed cycle sent %v, want none", lanes)
	}
}

func (f *fixture) useRuns(lane string, cycle int) {
	f.t.Helper()
	runs := f.t.TempDir()
	writeCycle(f.t, runs, cycle, []string{lane})
	f.over.Locate = RunsLocator(runs)
}

func TestOversight_AWARNSealKeepsTheDeliveryKnownSoALateEjectionAfterTheLaneLeftRecordsLaneLeft(t *testing.T) {
	t.Parallel()
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictWARN))
	f.endLane("A")

	f.send("ship", f.ejection(41, "d"))

	got := f.actions()
	if lanes, _ := b.snapshot(); len(lanes) != 0 || got[len(got)-1].Fields["actions"] != "lane_left" {
		t.Errorf("briefs %v, actions %+v, want lane_left as the last record", lanes, got)
	}
}

func TestOversight_ALandingBeforeTheSealLeavesNoDeliveryThatALateEjectionCanUse(t *testing.T) {
	t.Parallel()
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.endLane("A")
	f.send("ship", landedEvent(41))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	before := len(f.actions())

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 || len(f.actions()) != before {
		t.Errorf("an ejection of a cycle that landed sent %v and added %d records, want none", lanes, len(f.actions())-before)
	}
}

func TestOversightJourney_TheRunsLocatorLaneThatSealedFAILIsNotBriefedAndKeepsItsCellsFree(t *testing.T) {
	t.Parallel()
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 0)
	f.useRuns("A", 41)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictFAIL))

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 || !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Errorf("after a FAIL seal the ejection sent %v or held the cells again", lanes)
	}
}

func TestOversightJourney_TheRunsLocatorLaneThatLandedIsNotBriefedByALateEjection(t *testing.T) {
	t.Parallel()
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 0)
	f.useRuns("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	f.send("ship", landedEvent(41))

	f.send("ship", f.ejection(41, "d"))

	if lanes, _ := b.snapshot(); len(lanes) != 0 {
		t.Errorf("an ejection of a landed cycle sent %v, want none", lanes)
	}
}

func TestOversightJourney_TheRunsLocatorLaneThatSealedPASSIsBriefedAndFreedByItsLanding(t *testing.T) {
	t.Parallel()
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 0)
	f.useRuns("A", 41)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	f.send("ship", f.ejection(41, "d"))
	if lanes, _ := b.snapshot(); !reflect.DeepEqual(lanes, []string{"A"}) || f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Fatalf("briefs %v, want one brief to A and the cells held", lanes)
	}

	f.send("ship", landedEvent(41))

	if !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("after the landing, the cells of A are still held, want them free")
	}
}
