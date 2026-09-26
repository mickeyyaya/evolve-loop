package recovery

import (
	"strings"
	"testing"
)

func TestRecover_IntegrityAlwaysEscalates(t *testing.T) {
	t.Parallel()
	for _, in := range []RecoverInput{
		{Integrity: true},
		{Integrity: true, Busy: true},
		{Integrity: true, Cause: CauseDeadShell},
		{Integrity: true, Kind: "stuck_no_output", Attempts: 99, MaxAttempts: 6},
	} {
		d := Recover(in)
		if d.Action != ActionEscalate {
			t.Fatalf("integrity must always escalate, never auto-recover; in=%+v got %s", in, d.Action)
		}
		if !strings.Contains(d.Handler, "integrity") {
			t.Errorf("handler=%q must name the integrity link", d.Handler)
		}
	}
}

func TestRecover_OrderIsLoadBearing(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Busy: true, Cause: CauseModelInvalid})
	if d.Action != ActionExtend {
		t.Fatalf("busy must outrank known-fatal (never kill a working agent); got %s via %s", d.Action, d.Handler)
	}
	if d2 := Recover(RecoverInput{Integrity: true, Busy: true}); d2.Action != ActionEscalate {
		t.Fatalf("integrity must outrank busy; got %s", d2.Action)
	}
}

func TestRecover_KnownFatalNeverHitsLLM(t *testing.T) {
	t.Parallel()
	for _, cause := range []TerminalCause{CauseModelInvalid, CauseCLISelfUpdated, CauseDeadShell} {
		d := Recover(RecoverInput{Kind: "fatal_pane", Cause: cause})
		if d.Action != ActionKillRetry {
			t.Fatalf("known fatal cause %s must kill+retry deterministically; got %s", cause, d.Action)
		}
		if d.Action == ActionAdvise || strings.Contains(d.Handler, "advise") {
			t.Fatalf("known cause must NEVER reach the LLM tail (deterministic-first); handler=%s", d.Handler)
		}
		if d.Reason == "" {
			t.Error("every recovery decision carries a justification")
		}
	}
}

func TestRecover_StallWithinBudgetExtends(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Kind: "stuck_no_output", Cause: CauseUnknown, Attempts: 2, MaxAttempts: 6})
	if d.Action != ActionExtend {
		t.Fatalf("an unclassified stall within budget extends (the agent may be thinking); got %s via %s", d.Action, d.Handler)
	}
}

func TestRecover_UnknownEscalatesToAdvisor(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Kind: "stuck_no_output", Cause: CauseUnknown, Attempts: 6, MaxAttempts: 6})
	if d.Action != ActionAdvise {
		t.Fatalf("the unknown residue is the advisor's job; got %s via %s", d.Action, d.Handler)
	}
}

func TestChainStallPolicy_AdaptsToObserverVocabulary(t *testing.T) {
	t.Parallel()
	p := NewChainStallPolicy(6)
	a, reason := p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 600, ThresholdS: 600})
	if a != StallExtend || reason == "" {
		t.Fatalf("within-budget stall must extend with a reason; got %s %q", a, reason)
	}
	// IdleS/ThresholdS is the attempt count: 10 is past the budget of 6.
	a, _ = p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 6000, ThresholdS: 600})
	if a != StallEscalate {
		t.Fatalf("budget-exhausted unknown stall must escalate from the observer; got %s", a)
	}
}
