package core

// signal_test.go — ADR-0101 S1: the orchestrator is a registered listener of the
// Signal Center and the ADR-0044 C1 chokepoint (recordPhaseOutcome) is its
// first producer. Every terminal phase disposition, on both dispatch roots,
// becomes exactly one phase.outcome (or phase.aborted) event; a green cycle
// emits no WARN; a reasoned FAIL (cycle 1636's shape) is a WARN that names the
// phase, its code and its own reason — rendered by the stderr sink in the ONE
// line format, no longer hand-written at the chokepoint.

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestWithSignalCenter_WiresTheListener(t *testing.T) {
	t.Parallel()
	bare := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if bare.SignalCenterWired() {
		t.Error("no option → not wired (the nil Center is a test affordance, never the production default)")
	}
	if NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(nil)).SignalCenterWired() {
		t.Error("WithSignalCenter(nil) is not wired")
	}
	if s := bare.SignalSummary(); s.Total != 0 || s.Cycle != 0 || s.BySeverity == nil || s.ByKind == nil {
		t.Errorf("an unwired orchestrator reports an empty, usable summary: %+v", s)
	}
	c := signalcenter.New()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	if !o.SignalCenterWired() {
		t.Fatal("WithSignalCenter must report wired")
	}
	c.Emit(signalcenter.Event{Cycle: 3, Module: signalcenter.ModuleLoop, Origin: "Batch.run", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "wave"})
	if s := o.SignalSummary(); s.Cycle != 3 || s.Total != 1 {
		t.Errorf("the orchestrator's summary observes what the center delivers: %+v", s)
	}
}

func TestRunCycle_EveryDispatchedPhaseEmitsOnePhaseOutcome_GreenCycleStaysUnderTheWarnBudget(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "green"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	outcomes := 0
	for _, e := range got {
		if e.Kind != signalcenter.KindPhaseOutcome {
			continue
		}
		outcomes++
		if e.Module != signalcenter.ModuleOrchestrator || e.Origin != "Orchestrator.recordPhaseOutcome" || e.Cycle != res.Cycle ||
			e.Phase == "" || e.Attempt < 1 || e.Severity != signalcenter.SeverityInfo || e.Code != "" {
			t.Errorf("a green phase outcome is an INFO event with cycle/phase/attempt and no code: %+v", e)
		}
		if e.Fields["verdict"] != VerdictPASS || e.RunID == "" {
			t.Errorf("fields carry the verdict; the run id is stamped: %+v", e)
		}
	}
	if outcomes != len(res.PhasesRun) {
		t.Errorf("exactly one phase.outcome per dispatched phase: %d events for %d phases", outcomes, len(res.PhasesRun))
	}
	s := o.SignalSummary()
	if s.Cycle != res.Cycle || s.BySeverity[signalcenter.SeverityWarn] != 0 || s.BySeverity[signalcenter.SeverityIncident] != 0 {
		t.Errorf("the WARN budget for a green cycle is 0: %+v", s)
	}
}

func TestRunCycle_TriageFailIsAWarnOutcomeNamingTheReason(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	var stderr bytes.Buffer
	c.Subscribe(signalcenter.Filter(signalcenter.StderrSink(&stderr), signalcenter.SeverityWarn))
	var warn *signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) {
		if e.Severity == signalcenter.SeverityWarn && e.Kind == signalcenter.KindPhaseOutcome {
			ev := e
			warn = &ev
		}
	})
	runners := buildRunners(nil)
	runners[PhaseTriage] = diagnosticFailRunner{name: string(PhaseTriage), diags: []Diagnostic{{Severity: cyclestate.SeverityError, Message: protectedSurfaceRejection}}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithSignalCenter(c))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "protected"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if warn == nil {
		t.Fatal("triage's FAIL must surface as a WARN phase.outcome")
	}
	if warn.Code != CodePhaseVerdictFail || warn.Phase != string(PhaseTriage) || !strings.Contains(warn.Reason, "names protected surface") || warn.Fields["verdict"] != VerdictFAIL {
		t.Errorf("the event names the phase, its code and the phase's own reason: %+v", warn)
	}
	line := stderr.String()
	want := "[orchestrator] phase.outcome WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=" + strconv.Itoa(res.Cycle) + " phase=triage attempt=1"
	if !strings.HasPrefix(line, want) || !strings.Contains(line, "names protected surface") {
		t.Errorf("the ONE stderr line format replaces the hand-written one:\n got %s\nwant prefix %s", line, want)
	}
	if strings.Count(line, "\n") != 1 {
		t.Errorf("the WARN filter lets exactly this line through, got:\n%s", line)
	}
	if s := o.SignalSummary(); s.BySeverity[signalcenter.SeverityWarn] != 1 {
		t.Errorf("the summary counts the WARN: %+v", s)
	}
}

func TestRecordPhaseOutcome_AbortIsAPhaseAbortedWarn(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	result := CycleResult{Cycle: 41}
	var timings []phaseTimingEntry
	out := phaseOutcomeFrom(PhaseBuild, PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS}, 2, `tree-diff guard: phase "build" wrote to the main tree`, "")
	o.recordPhaseOutcome(&result, &timings, t.TempDir(), out)
	if len(got) != 1 {
		t.Fatalf("one event per outcome, got %d", len(got))
	}
	e := got[0]
	if e.Kind != signalcenter.KindPhaseAborted || e.Severity != signalcenter.SeverityWarn || e.Code != CodePhaseAborted ||
		e.Cycle != 41 || e.Attempt != 2 || !strings.Contains(e.Reason, "tree-diff guard") || e.Fields["verdict"] != VerdictPASS {
		t.Errorf("an abort after a PASS is phase.aborted WARN carrying both the verdict and the abort reason: %+v", e)
	}
}

func TestSignalSummary_FollowsTheCurrentCycle(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	st := &fakeStorage{state: State{LastCycleNumber: 9}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "one"}); err != nil {
		t.Fatal(err)
	}
	first := o.SignalSummary()
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "two"}); err != nil {
		t.Fatal(err)
	}
	second := o.SignalSummary()
	if first.Cycle != 10 || second.Cycle != 11 || second.Total != first.Total || first.Total == 0 {
		t.Errorf("the summary is the CURRENT cycle's only: first=%+v second=%+v", first, second)
	}
}

func TestRecordPhaseOutcome_WarnVerdictCarriesItsOwnErrorDiagnostics(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	result := CycleResult{Cycle: 42}
	var timings []phaseTimingEntry
	reasoned := PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictWARN, Diagnostics: []Diagnostic{
		{Severity: cyclestate.SeverityWarning, Message: "hygiene flag (not a reason)"},
		{Severity: cyclestate.SeverityError, Message: "coverage floor missed by 2%"},
	}}
	o.recordPhaseOutcome(&result, &timings, t.TempDir(), phaseOutcomeFrom(PhaseAudit, reasoned, 1, "", ""))
	bare := PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictWARN}
	o.recordPhaseOutcome(&result, &timings, t.TempDir(), phaseOutcomeFrom(PhaseAudit, bare, 2, "", ""))
	if len(got) != 2 {
		t.Fatalf("one event per outcome, got %d", len(got))
	}
	if e := got[0]; e.Kind != signalcenter.KindPhaseOutcome || e.Severity != signalcenter.SeverityWarn || e.Code != CodePhaseVerdictWarn ||
		e.Reason != "audit verdict=WARN: coverage floor missed by 2%" || e.Fields["verdict"] != VerdictWARN {
		t.Errorf("a WARN verdict is a WARN phase.outcome whose reason carries the phase's error-severity diagnostics only: %+v", e)
	}
	if e := got[1]; e.Code != CodePhaseVerdictWarn || e.Reason != "audit verdict=WARN" || e.Attempt != 2 {
		t.Errorf("a WARN with no error diagnostics keeps the bare reason: %+v", e)
	}
}

// signalMu exists for exactly this shape: cmd_loop reads the summary from its
// own goroutine while a drain delivers phase outcomes. Under -race a missing
// lock in observeSignal/SignalSummary is a hard failure.
func TestSignalSummary_IsSafeToReadWhileTheCenterDelivers(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 1; i <= 200; i++ {
			c.Emit(signalcenter.Event{Cycle: 5, Module: signalcenter.ModuleLoop, Origin: "Batch.run", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "wave " + strconv.Itoa(i)})
		}
	}()
	reads := 0
	for {
		select {
		case <-done:
			if s := o.SignalSummary(); s.Total != 200 || s.Cycle != 5 {
				t.Errorf("every delivered event is counted once the drain finishes: %+v", s)
			}
			if reads == 0 {
				t.Log("reader never overlapped the drain (timing); the lock is still exercised")
			}
			return
		default:
			_ = o.SignalSummary()
			reads++
		}
	}
}
