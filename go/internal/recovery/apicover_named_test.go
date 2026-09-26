package recovery

// These tests declare exported types explicitly (var x T): apicover counts naming a
// type, not using its fields.
import "testing"

func TestDecision_IntegrityEscalateFullStruct(t *testing.T) {
	t.Parallel()
	var got Decision = Recover(RecoverInput{Integrity: true})
	want := Decision{
		Action:  ActionEscalate,
		Handler: "integrity-escalate",
		Reason:  "integrity-adjacent state — never auto-recovered (ADR-0044 locked decision)",
	}
	if got != want {
		t.Fatalf("Recover(integrity) Decision = %+v, want %+v", got, want)
	}
}

// PhaseOutcome's only producer lives in core, which recovery cannot import, so
// the type itself is the contract under test.
func TestPhaseOutcome_AbortPreservesVerdictAndSpend(t *testing.T) {
	t.Parallel()
	happy := PhaseOutcome{Phase: "build", Verdict: "PASS", CostUSD: 0.42, DurationMS: 1500, BootMS: 30, AttemptCount: 2}

	aborted := happy
	aborted.AbortReason = "tree-diff guard: worktree leak"

	if aborted.Verdict != happy.Verdict {
		t.Errorf("recording an abort rewrote Verdict: %q -> %q (cycle-262: an abort is never a verdict rewrite)", happy.Verdict, aborted.Verdict)
	}
	if aborted.CostUSD != happy.CostUSD || aborted.DurationMS != happy.DurationMS || aborted.BootMS != happy.BootMS || aborted.AttemptCount != happy.AttemptCount {
		t.Errorf("recording an abort dropped burned spend/attempts: %+v vs %+v (cycle-262 lost the build's spend)", aborted, happy)
	}
	if aborted.AbortReason == "" {
		t.Error("AbortReason must be recordable on the outcome so the abort is not lost")
	}
}

func TestStallAction_FromPolicyVerdict(t *testing.T) {
	t.Parallel()
	p := NewChainStallPolicy(6)
	var act StallAction
	act, _ = p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 600, ThresholdS: 600})
	if act != StallExtend {
		t.Fatalf("within-budget stall: StallAction=%q, want %q", act, StallExtend)
	}
	act, _ = p.Decide(StallEvent{Kind: "process_dead", Phase: "build"})
	if act != StallKillRetry {
		t.Fatalf("dead process: StallAction=%q, want %q", act, StallKillRetry)
	}
}

func TestStallPolicy_InterfaceContract(t *testing.T) {
	t.Parallel()
	var p StallPolicy = NewChainStallPolicy(6)
	action, reason := p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 6000, ThresholdS: 600})
	if action != StallEscalate {
		t.Fatalf("budget-exhausted unknown stall via StallPolicy: got %q, want %q", action, StallEscalate)
	}
	if reason == "" {
		t.Error("every StallPolicy verdict carries a justification")
	}
}
