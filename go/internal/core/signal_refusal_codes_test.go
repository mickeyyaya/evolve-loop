package core

// signal_refusal_codes_test.go — the C1 chokepoint's phase.outcome WARN carries
// the phase's error-severity diagnostic CODES as fields.diagnostic_codes, so
// the console line and the durable stream name the class of a FAIL beside its
// prose ("… diagnostic_codes=TRIAGE_PROTECTED_SURFACE"); an uncoded FAIL
// carries no such field (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func triageFailOutcomeEvent(t *testing.T, diags []Diagnostic) *signalcenter.Event {
	t.Helper()
	c := signalcenter.New()
	var warn *signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindPhaseOutcome && e.Code == CodePhaseVerdictFail {
			ev := e
			warn = &ev
		}
	})
	runners := buildRunners(nil)
	runners[PhaseTriage] = diagnosticFailRunner{name: string(PhaseTriage), diags: diags}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithSignalCenter(c))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "codes"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if warn == nil {
		t.Fatal("triage's FAIL must surface as ORCHESTRATOR_PHASE_VERDICT_FAIL")
	}
	return warn
}

func TestEmitPhaseOutcome_FailCarriesTheDiagnosticCodes(t *testing.T) {
	warn := triageFailOutcomeEvent(t, []Diagnostic{
		{Severity: cyclestate.SeverityError, Message: protectedSurfaceRejection, Code: cyclestate.DiagCodeTriageProtectedSurface},
		{Severity: cyclestate.SeverityWarning, Message: "a trail", Code: "NOT_A_REASON"},
	})
	if got := warn.Fields["diagnostic_codes"]; got != cyclestate.DiagCodeTriageProtectedSurface {
		t.Errorf("fields.diagnostic_codes = %q, want the error-severity code only; fields = %v", got, warn.Fields)
	}
}

func TestEmitPhaseOutcome_UncodedFailCarriesNoCodesField(t *testing.T) {
	warn := triageFailOutcomeEvent(t, []Diagnostic{{Severity: cyclestate.SeverityError, Message: protectedSurfaceRejection}})
	if _, present := warn.Fields["diagnostic_codes"]; present {
		t.Errorf("an uncoded FAIL must not carry an empty diagnostic_codes field: %v", warn.Fields)
	}
}
