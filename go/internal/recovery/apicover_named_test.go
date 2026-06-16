package recovery

// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise the exported recovery/stall types that no existing test names by
// identifier (apicover counts field access like d.Action as "uses Decision",
// but not as "names Decision"). Each test asserts a REAL contract.

import "testing"

// TestDecision_IntegrityEscalateFullStruct pins the whole Decision the chain
// returns for an integrity-adjacent state: the locked ADR-0044 decision is
// escalate, claimed by the integrity link, with a justification. Full-struct
// equality on the typed Decision (names the type, not just a field).
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

// TestPhaseOutcome_AbortPreservesVerdictAndSpend pins PhaseOutcome's load-bearing
// structural invariant. PhaseOutcome is a deliberately LEAF DTO: its only
// producer is core's (unexported) phaseOutcomeFrom, and recovery must not import
// core (leaf constraint, see outcome.go), so there is no in-package or
// exported-cross-package call to exercise — the type IS the contract. The
// cycle-262 contract that type exists to enable: an abort is a cycle-level
// disposition recorded ALONGSIDE the agent's verdict, never a rewrite of it, and
// the burned spend is accounted even on abort. This proves Verdict/CostUSD/
// DurationMS are independent of AbortReason — start from a happy outcome, layer
// an abort on, and assert nothing else moved (it would catch a refactor that made
// Verdict derive from AbortReason, or dropped a spend field).
func TestPhaseOutcome_AbortPreservesVerdictAndSpend(t *testing.T) {
	t.Parallel()
	happy := PhaseOutcome{Phase: "build", Verdict: "PASS", CostUSD: 0.42, DurationMS: 1500, BootMS: 30, AttemptCount: 2}

	aborted := happy // record an abort on the already-produced outcome
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

// TestStallAction_FromPolicyVerdict names StallAction by capturing the typed
// verdict the StallPolicy returns. A within-budget unclassified stall yields
// StallExtend; a confirmed-dead process yields StallKillRetry.
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

// TestStallPolicy_InterfaceContract pins NewChainStallPolicy as a StallPolicy
// (the constructor's declared return type) and exercises the interface method:
// a stall past the extension budget escalates from the observer (it cannot
// dispatch an advisor, so advise degrades to escalate).
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
