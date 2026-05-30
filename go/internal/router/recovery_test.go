package router

import (
	"testing"
)

// TestRecover_Branches locks one case per handler branch in the recovery
// Chain of Responsibility.
func TestRecover_Branches(t *testing.T) {
	cases := []struct {
		name       string
		blocker    *Blocker
		wantPhase  string
		wantReason string
	}{
		// AUDIT_BINDING_* family → re-audit (saga re-establishes the binding).
		{"binding head moved", &Blocker{Code: "AUDIT_BINDING_HEAD_MOVED", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding tree mismatch", &Blocker{Code: "AUDIT_BINDING_TREE_MISMATCH", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding artifact sha", &Blocker{Code: "AUDIT_BINDING_ARTIFACT_SHA", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding artifact missing", &Blocker{Code: "AUDIT_BINDING_ARTIFACT_MISSING", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding verdict fail", &Blocker{Code: "AUDIT_BINDING_VERDICT_FAIL", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding malformed verdict", &Blocker{Code: "AUDIT_BINDING_MALFORMED_VERDICT", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding stale", &Blocker{Code: "AUDIT_BINDING_STALE", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding no auditor", &Blocker{Code: "AUDIT_BINDING_NO_AUDITOR", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},
		{"binding no ledger", &Blocker{Code: "AUDIT_BINDING_NO_LEDGER", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},

		// EGPS_RED_COUNT → re-audit (re-establish the gate precondition).
		{"egps red count", &Blocker{Code: "EGPS_RED_COUNT", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},

		// precondition class with a non-binding, non-egps code → re-audit.
		{"precondition generic", &Blocker{Code: "SOME_PRECONDITION", Class: "precondition", Stage: "ship"}, "audit", "recover:precondition-reaudit"},

		// AUDIT_BINDING_ prefix but empty class still routes via the prefix rule.
		{"binding prefix no class", &Blocker{Code: "AUDIT_BINDING_FUTURE_CODE", Class: "", Stage: "ship"}, "audit", "recover:precondition-reaudit"},

		// transient → retry ship.
		{"transient push rejected", &Blocker{Code: "GIT_PUSH_REJECTED", Class: "transient", Stage: "ship"}, "ship", "recover:transient-retry-ship"},
		{"transient git io", &Blocker{Code: "GIT_IO", Class: "transient", Stage: "ship"}, "ship", "recover:transient-retry-ship"},

		// integrity → block (end).
		{"integrity self sha", &Blocker{Code: "SELF_SHA_TAMPERED", Class: "integrity", Stage: "ship"}, PhaseEnd, "recover:integrity-block"},
		{"integrity tree drift", &Blocker{Code: "INTEGRITY_TREE_DRIFT", Class: "integrity", Stage: "ship"}, PhaseEnd, "recover:integrity-block"},

		// unknown code + unknown class → debugger (terminal catch-all).
		{"unknown novel code", &Blocker{Code: "SOME_NOVEL_CODE", Class: "", Stage: "ship"}, "debugger", "recover:unknown-debugger"},
		{"unknown class config", &Blocker{Code: "BAD_CONFIG", Class: "config", Stage: "ship"}, "debugger", "recover:unknown-debugger"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := RouteInput{Blocker: tc.blocker}
			got := Recover(in)
			if got.NextPhase != tc.wantPhase {
				t.Errorf("Recover(%s).NextPhase = %q, want %q", tc.name, got.NextPhase, tc.wantPhase)
			}
			if got.Reason != tc.wantReason {
				t.Errorf("Recover(%s).Reason = %q, want %q", tc.name, got.Reason, tc.wantReason)
			}
			if got.Evidence["code"] != tc.blocker.Code {
				t.Errorf("Recover(%s).Evidence[code] = %q, want %q", tc.name, got.Evidence["code"], tc.blocker.Code)
			}
			if got.Evidence["class"] != tc.blocker.Class {
				t.Errorf("Recover(%s).Evidence[class] = %q, want %q", tc.name, got.Evidence["class"], tc.blocker.Class)
			}
			if got.Evidence["stage"] != tc.blocker.Stage {
				t.Errorf("Recover(%s).Evidence[stage] = %q, want %q", tc.name, got.Evidence["stage"], tc.blocker.Stage)
			}
		})
	}
}

// TestRecover_NilBlocker locks the defensive no-blocker path.
func TestRecover_NilBlocker(t *testing.T) {
	got := Recover(RouteInput{})
	if got.NextPhase != PhaseEnd {
		t.Errorf("Recover(nil).NextPhase = %q, want %q", got.NextPhase, PhaseEnd)
	}
	if got.Reason != "recover:no-blocker" {
		t.Errorf("Recover(nil).Reason = %q, want %q", got.Reason, "recover:no-blocker")
	}
}

// TestRecover_OrderIntegrityWinsOverPrecondition proves the chain order:
// an error that is BOTH integrity class AND an AUDIT_BINDING_ code must hit
// the integrity-block rule first (→ end), never the precondition rule.
func TestRecover_OrderIntegrityWinsOverPrecondition(t *testing.T) {
	in := RouteInput{Blocker: &Blocker{
		Code:  "AUDIT_BINDING_HEAD_MOVED",
		Class: "integrity",
		Stage: "ship",
	}}
	got := Recover(in)
	if got.NextPhase != PhaseEnd {
		t.Errorf("integrity+binding NextPhase = %q, want %q (integrity must win)", got.NextPhase, PhaseEnd)
	}
	if got.Reason != "recover:integrity-block" {
		t.Errorf("integrity+binding Reason = %q, want %q (integrity must win)", got.Reason, "recover:integrity-block")
	}
}

// TestRecover_StrategyDelegationParity locks the shared-delegation invariant:
// StaticPreset and LLMProposal both delegate to the pure Recover, so for every
// input their Recover results are identical to each other and to Recover.
func TestRecover_StrategyDelegationParity(t *testing.T) {
	inputs := []RouteInput{
		{Blocker: &Blocker{Code: "AUDIT_BINDING_STALE", Class: "precondition", Stage: "ship"}},
		{Blocker: &Blocker{Code: "EGPS_RED_COUNT", Class: "precondition", Stage: "ship"}},
		{Blocker: &Blocker{Code: "GIT_PUSH_REJECTED", Class: "transient", Stage: "ship"}},
		{Blocker: &Blocker{Code: "SELF_SHA_TAMPERED", Class: "integrity", Stage: "ship"}},
		{Blocker: &Blocker{Code: "SOME_NOVEL_CODE", Class: "", Stage: "ship"}},
		{}, // nil blocker
	}

	var static StaticPreset
	var llm LLMProposal
	for _, in := range inputs {
		want := Recover(in)
		gotStatic := static.Recover(in)
		gotLLM := llm.Recover(in)
		if !decisionsEqual(gotStatic, want) {
			t.Errorf("StaticPreset.Recover = %+v, want %+v", gotStatic, want)
		}
		if !decisionsEqual(gotLLM, want) {
			t.Errorf("LLMProposal.Recover = %+v, want %+v", gotLLM, want)
		}
	}
}

func decisionsEqual(a, b RouterDecision) bool {
	if a.NextPhase != b.NextPhase || a.Reason != b.Reason {
		return false
	}
	if len(a.Evidence) != len(b.Evidence) {
		return false
	}
	for k, v := range a.Evidence {
		if b.Evidence[k] != v {
			return false
		}
	}
	return true
}
