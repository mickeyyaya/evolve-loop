package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func phasesOfDispatchedEvents(events []signalcenter.Event) []string {
	var phases []string
	for _, e := range eventsOfKind(events, signalcenter.KindPhaseDispatched) {
		phases = append(phases, e.Phase)
	}
	return phases
}

func assertDispatchedBeforeOutcome(t *testing.T, events []signalcenter.Event, cycle int, runID string) {
	t.Helper()
	outcomes := eventsOfKind(events, signalcenter.KindPhaseOutcome)
	if len(outcomes) == 0 {
		t.Fatal("the cycle recorded no phase outcome, so the order cannot be checked")
	}
	dispatchedOf := map[string]signalcenter.Event{}
	for _, e := range eventsOfKind(events, signalcenter.KindPhaseDispatched) {
		dispatchedOf[e.Phase] = e
	}
	for _, outcome := range outcomes {
		dispatched, ok := dispatchedOf[outcome.Phase]
		if !ok {
			t.Errorf("phase %s has an outcome and no phase.dispatched event", outcome.Phase)
			continue
		}
		if dispatched.Seq >= outcome.Seq {
			t.Errorf("phase %s: dispatched seq %d is not before its outcome seq %d", outcome.Phase, dispatched.Seq, outcome.Seq)
		}
		if dispatched.Cycle != cycle || dispatched.RunID != runID || dispatched.Module != signalcenter.ModuleOrchestrator || dispatched.Severity != signalcenter.SeverityInfo {
			t.Errorf("phase %s: dispatched event = %+v, want cycle %d, run %q, module orchestrator, INFO", outcome.Phase, dispatched, cycle, runID)
		}
	}
}

func TestDispatchPhase_EmitsPhaseDispatched(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "dispatched"})

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	dispatched := eventsOfKind(*got, signalcenter.KindPhaseDispatched)
	if len(dispatched) != len(res.PhasesRun) {
		t.Fatalf("phase.dispatched for %v, want one for each of the %d phases run: %v", phasesOfDispatchedEvents(*got), len(res.PhasesRun), res.PhasesRun)
	}
	if dispatched[0].RunID == "" || dispatched[0].Origin != "Orchestrator.emitPhaseDispatched" {
		t.Errorf("first dispatched event = %+v, want the cycle's run id and the emitter as origin", dispatched[0])
	}
	assertDispatchedBeforeOutcome(t, *got, res.Cycle, dispatched[0].RunID)
}

func TestDispatchPhase_ARetryOfOnePhaseEmitsOnePhaseDispatched(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	flaky := &fakeRunner{name: string(PhaseBuild), failErr: wrapTransient(85), failUntil: 1}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(map[Phase]string{}), WithSignalCenter(c))
	o.runners[PhaseBuild] = flaky

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "retry"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	count := 0
	for _, p := range phasesOfDispatchedEvents(*got) {
		if p == string(PhaseBuild) {
			count++
		}
	}
	if flaky.calls != 2 || count != 1 {
		t.Errorf("build ran %d times and has %d phase.dispatched events, want 2 runs and 1 event: the event marks the transition, not each attempt", flaky.calls, count)
	}
}

func TestResumeExecution_EmitsPhaseDispatched(t *testing.T) {
	t.Parallel()
	const runID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	st := &fakeStorage{
		state:      State{LastCycleNumber: 9},
		cycleState: CycleState{CycleID: 9, WorkspacePath: t.TempDir(), RunID: runID},
	}
	c, got := recordingCenter()
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 9})

	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if phases := phasesOfDispatchedEvents(*got); len(phases) == 0 || phases[0] != string(PhaseAudit) {
		t.Errorf("dispatched phases = %v, want the resumed cycle to start at audit", phases)
	}
	assertDispatchedBeforeOutcome(t, *got, 9, runID)
}

func TestDispatchEvaluateBatch_EmitsPhaseDispatchedForEachMember(t *testing.T) {
	t.Parallel()
	var active, maxActive int32
	runners := map[Phase]PhaseRunner{
		"tester":     &concurrentRunner{name: "tester", active: &active, maxActive: &maxActive},
		PhaseAudit:   &concurrentRunner{name: string(PhaseAudit), active: &active, maxActive: &maxActive},
		"reviewer-x": &concurrentRunner{name: "reviewer-x", active: &active, maxActive: &maxActive},
	}
	cr := newBatchCycleRun(t, runners, 2)
	c, got := recordingCenter()
	cr.o.signals = c

	if _, err := cr.dispatchEvaluateBatch([]Phase{"tester", PhaseAudit}); err != nil {
		t.Fatalf("dispatchEvaluateBatch: %v", err)
	}

	phases := phasesOfDispatchedEvents(*got)
	if len(phases) != 2 || !(phases[0] == "tester" && phases[1] == string(PhaseAudit)) {
		t.Errorf("dispatched phases = %v, want [tester audit] in batch order, and nothing for reviewer-x", phases)
	}
	for _, e := range eventsOfKind(*got, signalcenter.KindPhaseDispatched) {
		if e.Cycle != 1 || e.RunID != "r" {
			t.Errorf("event %+v, want cycle 1 and run r", e)
		}
	}
}
