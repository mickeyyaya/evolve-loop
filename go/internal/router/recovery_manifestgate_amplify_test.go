package router

import "testing"

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

// MANIFEST_GATE is never integrity-classed in production; this pins the chain order regardless.
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
