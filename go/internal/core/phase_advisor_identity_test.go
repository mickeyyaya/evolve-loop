package core

import "testing"

// TestAdvisorDispatch_DefaultsAndOverrides pins the WS1-S1 AgentIdentity value
// object (ADR-0052): the default identity a fresh PhaseAdvisor dispatches under,
// and that the functional options populate that ONE identity (which advisorLaunch
// then reads to build BridgeRequest). It asserts the value object directly —
// TestPhaseAdvisor_DispatchWiringFlowsToBridge already covers the field→bridge
// flow — so the single-source identity both control-plane advisors now share is
// locked against drift.
func TestAdvisorDispatch_DefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	// Default identity: deep claude on the tmux driver, labeled "router".
	// Profile/Persona empty (derived per-call / legacy framing).
	def := NewPhaseAdvisor(&fakeBridge{}).identity
	if want := (AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"}); def != want {
		t.Fatalf("default PhaseAdvisor identity = %+v, want %+v", def, want)
	}

	// Options populate the SAME identity value object; AgentLabel is identity,
	// not a per-call param, so it stays "router" regardless of overrides.
	got := NewPhaseAdvisor(&fakeBridge{},
		WithProposerCLI("codex-tmux"),
		WithProposerModel("gpt-5.5"),
		WithPersona("PERSONA_BODY"),
	).identity
	if got.CLI != "codex-tmux" || got.Model != "gpt-5.5" || got.Persona != "PERSONA_BODY" || got.AgentLabel != "router" {
		t.Errorf("overrides did not populate identity correctly: %+v", got)
	}

	// FailureAdvisor shares the value object with a distinct label.
	fdef := NewFailureAdvisor(&fakeBridge{}).identity
	if want := (AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "failure-advisor"}); fdef != want {
		t.Fatalf("default FailureAdvisor identity = %+v, want %+v", fdef, want)
	}

	// The identity reaches the bridge on a Plan launch (end-to-end).
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	if _, err := NewPhaseAdvisor(fb, WithProposerCLI("agy"), WithProposerModel("gemini-3.5-flash")).Plan(baseRouteInput()); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if fb.gotReq.CLI != "agy" || fb.gotReq.Model != "gemini-3.5-flash" || fb.gotReq.Agent != "router" {
		t.Errorf("identity did not flow to BridgeRequest: CLI=%q Model=%q Agent=%q", fb.gotReq.CLI, fb.gotReq.Model, fb.gotReq.Agent)
	}
}
