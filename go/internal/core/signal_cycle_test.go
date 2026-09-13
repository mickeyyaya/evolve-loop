package core

// signal_cycle_test.go — ADR-0101 S2a producers: cycle.sealed and
// system.failure at completeCycle (both roots) and, for an abnormal exit,
// from the epilogue; ship.error at recordShipError; quota.paused at
// pauseForQuota (the seam both roots reach). Each is an Adapter from a typed
// value the pipeline already owns to one Event — no new state, no decision.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordingCenter() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

func eventsOfKind(got []signalcenter.Event, kind signalcenter.Kind) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range got {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func TestRunCycle_SealsTheCycleWithOneCycleSealedEventAfterEveryPhaseOutcome(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "sealed"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	sealed := eventsOfKind(*got, signalcenter.KindCycleSealed)
	if len(sealed) != 1 {
		t.Fatalf("exactly one cycle.sealed per completed cycle, got %d", len(sealed))
	}
	e := sealed[0]
	if e.Severity != signalcenter.SeverityInfo || e.Code != "" || e.Cycle != res.Cycle || e.Origin != "cycleRun.completeCycle" || e.Module != signalcenter.ModuleOrchestrator || e.RunID == "" {
		t.Errorf("a green closeout is INFO from the shared closeout, stamped with the run id: %+v", e)
	}
	if e.Fields["final_verdict"] != res.FinalVerdict || e.Fields["phases_run"] != strconv.Itoa(len(res.PhasesRun)) || e.Reason != "final verdict "+res.FinalVerdict {
		t.Errorf("the seal names the final verdict and the phase count: %+v", e)
	}
	for _, o := range eventsOfKind(*got, signalcenter.KindPhaseOutcome) {
		if e.Seq < o.Seq {
			t.Errorf("the seal is the LAST orchestrator event of the cycle: seq %d < outcome %d", e.Seq, o.Seq)
		}
	}
}

func TestEmitCycleClose_SystemFailureIsAnIncidentWhenHaltingAndAWarnOtherwise(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	cr := &cycleRun{o: o, cycle: 862, cs: CycleState{RunID: "run-862"}}
	halting := CycleResult{Cycle: 862, FinalVerdict: VerdictFAIL, TerminationReason: "audit-fail-floor", SystemFailure: &SystemFailureSignal{
		Category: "verdict-incoherence", Level: "system", Evidence: "recorded FAIL but audit=PASS acs=PASS", Halt: true}}
	cr.emitCycleClose(halting, "cycleRun.completeCycle")
	if len(*got) != 2 {
		t.Fatalf("system.failure then cycle.sealed, got %d events: %+v", len(*got), *got)
	}
	sf, sealed := (*got)[0], (*got)[1]
	if sf.Kind != signalcenter.KindSystemFailure || sf.Severity != signalcenter.SeverityIncident || sf.Code != CodeSystemFailure ||
		sf.Fields["category"] != "verdict-incoherence" || sf.Fields["halt"] != "true" || !strings.Contains(sf.Reason, "recorded FAIL but audit=PASS") || sf.Cycle != 862 || sf.RunID != "run-862" {
		t.Errorf("a halting system failure is an INCIDENT naming category and evidence, stamped with the cycle's run id: %+v", sf)
	}
	if sealed.Kind != signalcenter.KindCycleSealed || sealed.Severity != signalcenter.SeverityWarn || sealed.Code != CodeCycleFailed ||
		sealed.Fields["final_verdict"] != VerdictFAIL || sealed.Fields["termination_reason"] != "audit-fail-floor" || sealed.RunID != "run-862" {
		t.Errorf("a FAIL seal is a WARN with its own code and the termination reason: %+v", sealed)
	}
	*got = nil
	cr.cycle = 1535
	cr.emitCycleClose(CycleResult{Cycle: 1535, FinalVerdict: "SKIPPED_LANDING_LOST", SystemFailure: &SystemFailureSignal{Category: "landing-lost", Level: "system", Evidence: "ship ran, landing lost", Halt: false}}, "cycleRun.completeCycle")
	if len(*got) != 2 || (*got)[0].Severity != signalcenter.SeverityWarn || (*got)[0].Fields["halt"] != "false" || (*got)[0].Cycle != 1535 {
		t.Errorf("a non-halting system failure is a WARN: %+v", *got)
	}
}

func TestRecordShipError_EmitsShipErrorUnderTheShipModuleWithTheProjectedCode(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	cs := CycleState{WorkspacePath: t.TempDir(), Phase: string(PhaseShip), RunID: "run-1632"}
	se := NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "main moved during the landing", "attempt", "1")
	o.recordShipError(context.Background(), 1632, cs, se)
	if len(*got) != 1 {
		t.Fatalf("one ship.error per recorded ship error, got %d", len(*got))
	}
	e := (*got)[0]
	if e.Module != signalcenter.ModuleShip || e.Kind != signalcenter.KindShipError || e.Severity != signalcenter.SeverityWarn ||
		e.Code != "SHIP_GIT_FLEET_REBASE_NEEDED" || e.Cycle != 1632 || e.Phase != "ship" || e.Origin != "Orchestrator.recordShipError" || e.RunID != "run-1632" ||
		e.Reason != "main moved during the landing" || e.Fields["class"] != "transient" || e.Fields["stage"] != "atomic-ship" || e.Fields["path"] == "" {
		t.Errorf("the event projects the ShipError verbatim (code, class, stage, message, artifact path): %+v", e)
	}
	*got = nil
	o.recordShipError(context.Background(), 1, cs, NewShipError(CodeIntegrityTreeDrift, ShipClassIntegrity, StagePostShip, "tree drifted"))
	if len(*got) != 1 || (*got)[0].Severity != signalcenter.SeverityIncident {
		t.Errorf("the integrity class is an INCIDENT (§5.3, ShipErrorClass.SignalSeverity): %+v", *got)
	}
}

func TestPauseForQuota_EmitsQuotaPausedFromTheSeamBothRootsShare(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	cr := &cycleRun{o: o, ctx: context.Background(), cycle: 1601, req: CycleRequest{ProjectRoot: t.TempDir()},
		cs: CycleState{Phase: string(PhaseBuild), WorkspacePath: t.TempDir(), RunID: "run-1601"}}
	err := cr.pauseForQuota(PhaseBuild, PhaseResponse{Phase: string(PhaseBuild)}, 3)
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("the pause keeps its typed sentinel: %v", err)
	}
	paused := eventsOfKind(*got, signalcenter.KindQuotaPaused)
	if len(paused) != 1 {
		t.Fatalf("exactly one quota.paused per pause, got %d: %+v", len(paused), *got)
	}
	e := paused[0]
	if e.Severity != signalcenter.SeverityWarn || e.Code != CodeQuotaPaused || e.Cycle != 1601 || e.Phase != "build" || e.RunID != "run-1601" ||
		e.Origin != "cycleRun.pauseForQuota" || !strings.Contains(e.Reason, "exit=85 across 3 attempts") || e.Fields["phase"] != "build" {
		t.Errorf("a quota pause is a WARN naming the paused phase with the one error text the ledger carries: %+v", e)
	}
	before := len(*got)
	cr.abnormalEpilogue(err)
	if len(*got) != before {
		t.Errorf("the epilogue adds nothing for a pause (no seal, no second quota event): %+v", (*got)[before:])
	}
}

func TestRunCycle_QuotaExhaustionOnTheFreshRootEmitsQuotaPausedAndNoSeal(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	runners := buildRunners(nil)
	runners[PhaseBuild] = &fakeRunner{name: string(PhaseBuild), failErr: wrapTransient(85), failUntil: 99}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithSignalCenter(c))
	o.retryConfig.RetryBackoffBaseS = 0
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "quota"})
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("the fresh root pauses with the typed sentinel: %v", err)
	}
	if paused := eventsOfKind(*got, signalcenter.KindQuotaPaused); len(paused) != 1 || paused[0].Phase != "build" {
		t.Errorf("the fresh root emits exactly one quota.paused for the paused phase: %+v", paused)
	}
	if sealed := eventsOfKind(*got, signalcenter.KindCycleSealed); len(sealed) != 0 {
		t.Errorf("a pause is not a seal: %+v", sealed)
	}
}

func TestAbnormalEpilogue_AbnormalExitSealsTheCycleAsFAILWithTheAbortReason(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	cr := &cycleRun{o: o, ctx: context.Background(), cycle: 1208, req: CycleRequest{ProjectRoot: t.TempDir()},
		cs: CycleState{Phase: string(PhaseBuild), WorkspacePath: t.TempDir(), RunID: "run-1208"}}
	cr.abnormalEpilogue(errors.New("bridge died mid-phase"))
	sealed := eventsOfKind(*got, signalcenter.KindCycleSealed)
	if len(sealed) != 1 {
		t.Fatalf("an abnormal exit seals the cycle exactly once, got %d: %+v", len(sealed), *got)
	}
	e := sealed[0]
	if e.Severity != signalcenter.SeverityWarn || e.Code != CodeCycleFailed || e.Fields["final_verdict"] != VerdictFAIL || e.Cycle != 1208 || e.RunID != "run-1208" ||
		e.Origin != "cycleRun.abnormalEpilogue" ||
		!strings.Contains(e.Fields["termination_reason"], "build") || !strings.Contains(e.Fields["termination_reason"], "bridge died") {
		t.Errorf("the seal is FAIL with the abort reason (phase + cause) as the termination reason, from the epilogue's own origin: %+v", e)
	}
	*got = nil
	cr.cycleCompletedNormally = true
	cr.abnormalEpilogue(nil)
	if len(*got) != 0 {
		t.Errorf("a normal completion emits nothing from the epilogue: %+v", *got)
	}
}
