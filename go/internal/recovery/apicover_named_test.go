package recovery

import (
	"reflect"
	"testing"
)

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

func TestPhaseOutcome_AbortPreservesVerdictAndSpend(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(PhaseOutcome{})
	for _, field := range []string{"Verdict", "CostUSD", "DurationMS", "BootMS", "AttemptCount", "AbortReason"} {
		if _, ok := typ.FieldByName(field); !ok {
			t.Errorf("PhaseOutcome lost field %s: an abort is recorded beside the verdict and the burned spend, never instead of them (cycle-262)", field)
		}
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
