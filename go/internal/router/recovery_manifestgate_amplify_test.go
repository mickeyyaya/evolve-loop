package router

import "testing"

// TestRecover_ManifestGate_RoutesToDebugger amplifies cycle-1064's
// manifest-gate-policy-wiring change: recovery.go's shipLocalCodes now
// carries "MANIFEST_GATE" (mirroring COMMIT_PREFIX_GATE), but no test in the
// router package exercised the actual Recover() routing decision — the
// composed path a ledger/debugger consumer actually sees. A manifest-gate
// block is a ship-LOCAL precondition a re-audit can never re-establish, so it
// must route to the debugger, never back to audit (the cycle-230 loop).
func TestRecover_ManifestGate_RoutesToDebugger(t *testing.T) {
	in := RouteInput{Blocker: &Blocker{Code: "MANIFEST_GATE", Class: "precondition", Stage: "ship"}}
	got := Recover(in)
	if got.NextPhase != "debugger" {
		t.Errorf("NextPhase = %q, want %q", got.NextPhase, "debugger")
	}
	if got.Reason != "recover:ship-local-debugger" {
		t.Errorf("Reason = %q, want %q", got.Reason, "recover:ship-local-debugger")
	}
	if got.Evidence["code"] != "MANIFEST_GATE" {
		t.Errorf("Evidence[code] = %q, want %q", got.Evidence["code"], "MANIFEST_GATE")
	}
}

// TestRecover_ManifestGate_IntegrityClassStillWinsChainOrder locks the
// Chain-of-Responsibility ordering invariant (proven pre-existing for
// AUDIT_BINDING_* codes in TestRecover_OrderIntegrityWinsOverPrecondition)
// also holds for the new code: an integrity-classed error must hit the
// integrity-block rule before any code-specific ship-local lookup, even
// though MANIFEST_GATE is never constructed with class=integrity in
// production today. A ship-local map keyed purely on Code (ignoring Class)
// would break this ordering silently.
func TestRecover_ManifestGate_IntegrityClassStillWinsChainOrder(t *testing.T) {
	in := RouteInput{Blocker: &Blocker{Code: "MANIFEST_GATE", Class: "integrity", Stage: "ship"}}
	got := Recover(in)
	if got.NextPhase != PhaseEnd {
		t.Errorf("NextPhase = %q, want %q (integrity must win)", got.NextPhase, PhaseEnd)
	}
	if got.Reason != "recover:integrity-block" {
		t.Errorf("Reason = %q, want %q (integrity must win)", got.Reason, "recover:integrity-block")
	}
}

// TestRecover_ManifestGate_StrategyDelegationParity extends the shared
// StaticPreset/LLMProposal delegation-parity invariant
// (TestRecover_StrategyDelegationParity) to the new code: both strategy
// wrappers must resolve MANIFEST_GATE identically to the pure Recover, so an
// LLM-proposed route can never diverge from the deterministic static one for
// a ship-LOCAL precondition.
func TestRecover_ManifestGate_StrategyDelegationParity(t *testing.T) {
	in := RouteInput{Blocker: &Blocker{Code: "MANIFEST_GATE", Class: "precondition", Stage: "ship"}}
	want := Recover(in)

	var static StaticPreset
	var llm LLMProposal
	if got := static.Recover(in); !decisionsEqual(got, want) {
		t.Errorf("StaticPreset.Recover = %+v, want %+v", got, want)
	}
	if got := llm.Recover(in); !decisionsEqual(got, want) {
		t.Errorf("LLMProposal.Recover = %+v, want %+v", got, want)
	}
}
