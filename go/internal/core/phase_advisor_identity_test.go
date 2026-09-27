package core

import "testing"

// TestAdvisorDispatch_DefaultsAndOverrides asserts the AgentIdentity value
// object directly — TestPhaseAdvisor_DispatchWiringFlowsToBridge already
// covers the field→bridge flow — so the single-source identity both
// control-plane advisors share is locked against drift.
func TestAdvisorDispatch_DefaultsAndOverrides(t *testing.T) {
	t.Parallel()

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

	fdef := NewFailureAdvisor(&fakeBridge{}).identity
	if want := (AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "failure-advisor"}); fdef != want {
		t.Fatalf("default FailureAdvisor identity = %+v, want %+v", fdef, want)
	}

	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	if _, err := NewPhaseAdvisor(fb, WithProposerCLI("agy"), WithProposerModel("gemini-3.5-flash")).Plan(baseRouteInput()); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if fb.gotReq.CLI != "agy" || fb.gotReq.Model != "gemini-3.5-flash" || fb.gotReq.Agent != "router" {
		t.Errorf("identity did not flow to BridgeRequest: CLI=%q Model=%q Agent=%q", fb.gotReq.CLI, fb.gotReq.Model, fb.gotReq.Agent)
	}
}
