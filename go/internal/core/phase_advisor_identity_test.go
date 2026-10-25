package core

import "testing"

func TestAdvisorDispatch_DefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	def := NewPhaseAdvisor(&fakeBridge{}).identity
	if want := (AgentIdentity{CLI: "claude-tmux", Model: "deep", AgentLabel: "router"}); def != want {
		t.Fatalf("default PhaseAdvisor identity = %+v, want %+v", def, want)
	}

	got := NewPhaseAdvisor(&fakeBridge{},
		WithProposerCLI("codex-tmux"),
		WithProposerModel("gpt-5.5"),
		WithPersona("PERSONA_BODY"),
	).identity
	if got.CLI != "codex-tmux" || got.Model != "gpt-5.5" || got.Persona != "PERSONA_BODY" || got.AgentLabel != "router" {
		t.Errorf("overrides did not populate identity correctly: %+v", got)
	}

	fdef := NewFailureAdvisor(&fakeBridge{}).identity
	if want := (AgentIdentity{CLI: "claude-tmux", Model: "deep", AgentLabel: "failure-advisor"}); fdef != want {
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
