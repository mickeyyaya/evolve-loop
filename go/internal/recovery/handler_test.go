package recovery

// handler_test.go — ADR-0044 C3 (Slice 6) RED tests: the recovery Chain of
// Responsibility — the single owner that turns a classified terminal state
// into a typed, justified recovery action. Modeled on router/recovery.go's
// chain (the repo's proven CoR shape). Order is load-bearing:
//
//   integrity-escalate → busy-extend → known-fatal-kill →
//   stall-within-budget-extend → unknown-advise (terminal)
//
// Integrity never auto-recovers; a Busy (visibly working) agent is never
// killed; a KNOWN fatal cause kills immediately (zero LLM tokens — the
// deterministic registry already classified it); only the UNKNOWN residue
// reaches the LLM failure-advisor tail.

import (
	"strings"
	"testing"
)

func TestRecover_IntegrityAlwaysEscalates(t *testing.T) {
	t.Parallel()
	// Integrity outranks EVERYTHING — even a busy pane, even a known cause.
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
	// Busy outranks a known-fatal cause: a visibly-working agent whose
	// scrollback happens to contain a fatal signature is never killed.
	d := Recover(RecoverInput{Busy: true, Cause: CauseModelInvalid})
	if d.Action != ActionExtend {
		t.Fatalf("busy must outrank known-fatal (never kill a working agent); got %s via %s", d.Action, d.Handler)
	}
	// And integrity outranks busy (pinned above, asserted here as the pair).
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
	// Budget exhausted + no deterministic classification → the LLM tail.
	d := Recover(RecoverInput{Kind: "stuck_no_output", Cause: CauseUnknown, Attempts: 6, MaxAttempts: 6})
	if d.Action != ActionAdvise {
		t.Fatalf("the unknown residue is the advisor's job; got %s via %s", d.Action, d.Handler)
	}
}

// TestChainStallPolicy_AdaptsToObserverVocabulary pins the C4 bridge: the
// chain exposed as a StallPolicy maps Action→StallAction (advise/escalate
// both surface as escalate — the observer cannot dispatch an advisor).
func TestChainStallPolicy_AdaptsToObserverVocabulary(t *testing.T) {
	t.Parallel()
	p := NewChainStallPolicy(6)
	// Within budget (idle 1× threshold) → extend.
	a, reason := p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 600, ThresholdS: 600})
	if a != StallExtend || reason == "" {
		t.Fatalf("within-budget stall must extend with a reason; got %s %q", a, reason)
	}
	// Past budget (idle 10× threshold, maxExtends 6) → escalate (the
	// observer surfaces; it never advises directly).
	a, _ = p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 6000, ThresholdS: 600})
	if a != StallEscalate {
		t.Fatalf("budget-exhausted unknown stall must escalate from the observer; got %s", a)
	}
}
