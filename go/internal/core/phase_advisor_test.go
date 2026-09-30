package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type fakeBridge struct {
	stdout     string
	err        error
	durationMS int64
	tokens     TokenUsage // the span golden threads token usage
	gotReq     BridgeRequest
	calls      int
}

func (f *fakeBridge) Launch(_ context.Context, req BridgeRequest) (BridgeResponse, error) {
	f.calls++
	f.gotReq = req
	if f.err != nil {
		return BridgeResponse{}, f.err
	}
	return BridgeResponse{Stdout: f.stdout, ExitCode: 0, DurationMS: f.durationMS, Tokens: f.tokens}, nil
}
func (f *fakeBridge) Probe(_ context.Context) (BridgeProbe, error) { return BridgeProbe{}, nil }

func TestAdvisorLaunch_ThreadsActiveWorktree(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	adv := NewPhaseAdvisor(fb)
	in := baseRouteInput()
	in.ActiveWorktree = "/wt/cycle-7"
	if _, err := adv.Plan(in); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if fb.gotReq.Worktree != "/wt/cycle-7" {
		t.Errorf("advisor bridge launch did not thread the worktree: BridgeRequest.Worktree=%q, want %q (empty trips the EVOLVE_FLEET worktree guard)", fb.gotReq.Worktree, "/wt/cycle-7")
	}
}

func TestAdvisorPlanInput_ThreadsActiveWorktree(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{now: func() time.Time { return time.Time{} }}
	in := o.advisorPlanInput(context.Background(), "build", router.RoutingSignals{}, CycleRequest{}, State{}, CycleState{ActiveWorktree: "/wt/cycle-9"}, 9, nil, nil)
	if in.ActiveWorktree != "/wt/cycle-9" {
		t.Errorf("advisorPlanInput did not thread cs.ActiveWorktree into RouteInput: got %q, want /wt/cycle-9", in.ActiveWorktree)
	}
}

func baseRouteInput() router.RouteInput {
	return router.RouteInput{
		Current:     "build",
		Verdict:     VerdictPASS,
		Workspace:   "/tmp/ws",
		ProjectRoot: "/proj",
		Cycle:       7,
		Env:         map[string]string{"EVOLVE_CLI": "claude-tmux"},
	}
}

func TestPhaseAdvisor_ParsesValidJSON(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `{"next_phase":"tester","insert_phases":["tester"],"justification":"acs red"}`}
	p := NewPhaseAdvisor(fb)
	prop, err := p.Propose(baseRouteInput())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if prop.NextPhase != "tester" || len(prop.InsertPhases) != 1 || prop.InsertPhases[0] != "tester" {
		t.Errorf("proposal=%+v, want next=tester insert=[tester]", prop)
	}
	if !strings.HasSuffix(fb.gotReq.Profile, "/.evolve/profiles/router.json") {
		t.Errorf("profile=%q, want .../.evolve/profiles/router.json", fb.gotReq.Profile)
	}
	if !strings.HasSuffix(fb.gotReq.ArtifactPath, "routing-proposal.json") {
		t.Errorf("artifact=%q, want .../routing-proposal.json", fb.gotReq.ArtifactPath)
	}
	if fb.gotReq.Cycle != 7 {
		t.Errorf("cycle=%d, want 7", fb.gotReq.Cycle)
	}
}

func TestPhaseAdvisor_SelectsContractForEachRouterProtocol(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		contract   string
		artifact   string
		completion CompletionContract
		launch     func(*PhaseAdvisor, router.RouteInput) error
	}{
		{
			name: "plan", stdout: `[{"phase":"scout","run":true}]`,
			contract: "router", artifact: "routing-plan.json", completion: "artifact",
			launch: func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.Plan(in); return err },
		},
		{
			name: "replan", stdout: `[{"phase":"audit","run":true}]`,
			contract: "router-replan", artifact: "routing-replan.json", completion: "artifact",
			launch: func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.RePlan(in); return err },
		},
		{
			name: "proposal", stdout: `{"next_phase":"audit"}`,
			contract: "router-proposal", artifact: "routing-proposal.json", completion: "artifact",
			launch: func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.Propose(in); return err },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := &fakeBridge{stdout: tt.stdout}
			in := baseRouteInput()
			in.Workspace = t.TempDir()
			if err := tt.launch(NewPhaseAdvisor(fb), in); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if fb.gotReq.Agent != "router" {
				t.Errorf("Agent=%q, want router", fb.gotReq.Agent)
			}
			if fb.gotReq.Contract != tt.contract {
				t.Errorf("Contract=%q, want %q", fb.gotReq.Contract, tt.contract)
			}
			if !strings.HasSuffix(fb.gotReq.ArtifactPath, tt.artifact) {
				t.Errorf("ArtifactPath=%q, want suffix %q", fb.gotReq.ArtifactPath, tt.artifact)
			}
			if fb.gotReq.Completion != tt.completion {
				t.Errorf("Completion=%q, want %q", fb.gotReq.Completion, tt.completion)
			}
		})
	}
}

func TestPhaseAdvisor_TolerantOfFenceAndProse(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: "Here is my routing call:\n```json\n{\"next_phase\":\"audit\",\"justification\":\"done\"}\n```\nThanks!"}
	prop, err := NewPhaseAdvisor(fb).Propose(baseRouteInput())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if prop.NextPhase != "audit" {
		t.Errorf("next=%q, want audit", prop.NextPhase)
	}
}

func TestPhaseAdvisor_FailSafe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fb   *fakeBridge
		in   router.RouteInput
	}{
		{"bridge error", &fakeBridge{err: errors.New("boom")}, baseRouteInput()},
		{"no json", &fakeBridge{stdout: "I could not decide."}, baseRouteInput()},
		{"empty proposal", &fakeBridge{stdout: `{"justification":"nothing"}`}, baseRouteInput()},
	}
	for _, c := range cases {
		if _, err := NewPhaseAdvisor(c.fb).Propose(c.in); err == nil {
			t.Errorf("%s: want error (so LLMProposal degrades to static), got nil", c.name)
		}
	}
	if _, err := NewPhaseAdvisor(nil).Propose(baseRouteInput()); err == nil {
		t.Error("nil bridge: want error")
	}
	noWs := baseRouteInput()
	noWs.Workspace = ""
	if _, err := NewPhaseAdvisor(&fakeBridge{stdout: "{}"}).Propose(noWs); err == nil {
		t.Error("empty workspace: want error")
	}
}

func TestPhaseAdvisor_ProposalIsClampedByKernel(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `{"next_phase":"ship","justification":"skip audit"}`}
	strat := router.LLMProposal{Proposer: NewPhaseAdvisor(fb)}

	in := baseRouteInput()
	in.Cfg = config.RoutingConfig{
		Stage:         config.StageEnforce,
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
		Triggers:      map[string]config.RoutingBlock{},
	}
	in.Completed = []string{"scout", "build"}

	dec := strat.Decide(in)
	if dec.NextPhase != "audit" {
		t.Errorf("NextPhase=%q, want audit (kernel forces audit before ship)", dec.NextPhase)
	}
	foundClamp := false
	for _, c := range dec.Clamps {
		if c.Rule == "llm-proposal-clamped" && c.Proposed == "ship" && c.Forced == "audit" {
			foundClamp = true
		}
	}
	if !foundClamp {
		t.Errorf("expected llm-proposal-clamped(ship->audit), clamps=%+v", dec.Clamps)
	}
	if fb.calls != 1 {
		t.Errorf("bridge calls=%d, want 1", fb.calls)
	}
}

func TestPhaseAdvisor_PlanParsesArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		stdout       string
		wantLen      int
		wantScoutRun bool
	}{
		{"bare array, run+skip mix", `[{"phase":"scout","run":true,"justification":"fresh discovery"},{"phase":"triage","run":false,"justification":"carryover already queued"}]`, 2, true},
		{"fenced", "```json\n[{\"phase\":\"scout\",\"run\":false,\"justification\":\"backlog queued\"}]\n```", 1, false},
		{"leading + trailing prose", "Here is the plan:\n[{\"phase\":\"scout\",\"run\":true,\"justification\":\"new work\"}]\nThanks!", 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fb := &fakeBridge{stdout: c.stdout}
			plan, err := NewPhaseAdvisor(fb).Plan(baseRouteInput())
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if len(plan.Entries) != c.wantLen {
				t.Fatalf("entries=%d, want %d (%+v)", len(plan.Entries), c.wantLen, plan.Entries)
			}
			if plan.Entries[0].Phase != "scout" || plan.Entries[0].Run != c.wantScoutRun {
				t.Errorf("first entry=%+v, want scout run=%v", plan.Entries[0], c.wantScoutRun)
			}
			if !strings.HasSuffix(fb.gotReq.ArtifactPath, "routing-plan.json") {
				t.Errorf("artifact=%q, want .../routing-plan.json", fb.gotReq.ArtifactPath)
			}
			if fb.gotReq.Completion != "artifact" {
				t.Errorf("Completion=%q, want artifact", fb.gotReq.Completion)
			}
		})
	}
}

func TestPhaseAdvisor_PersonaComposition(t *testing.T) {
	t.Parallel()
	plan := `[{"phase":"scout","run":true,"justification":"x"}]`

	t.Run("persona used + dynamic context appended", func(t *testing.T) {
		fb := &fakeBridge{stdout: plan}
		adv := NewPhaseAdvisor(fb, WithPersona("PERSONA_MARKER_42"))
		if _, err := adv.Plan(router.RouteInput{Workspace: "/tmp/x", Cycle: 7}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(fb.gotReq.Prompt, "PERSONA_MARKER_42") {
			t.Error("prompt must include the injected persona body")
		}
		if !strings.Contains(fb.gotReq.Prompt, "# This cycle") {
			t.Error("prompt must append the dynamic per-cycle context after the persona")
		}
	})

	t.Run("no persona falls back to inline framing", func(t *testing.T) {
		fb := &fakeBridge{stdout: plan}
		adv := NewPhaseAdvisor(fb)
		if _, err := adv.Plan(router.RouteInput{Workspace: "/tmp/x", Cycle: 7}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(fb.gotReq.Prompt, "PHASE ADVISOR") {
			t.Error("fallback prompt must use the legacy inline framing")
		}
	})
}

func TestPhaseAdvisor_PlanPromptUsesAbsoluteArtifactPath(t *testing.T) {
	t.Parallel()
	const ws = "/tmp/ws-abs-artifact-test"
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	if _, err := NewPhaseAdvisor(fb, WithPersona("PERSONA")).Plan(router.RouteInput{Workspace: ws, Cycle: 7}); err != nil {
		t.Fatal(err)
	}
	want := ws + "/routing-plan.json" // absolute — must equal advisorLaunch's watched ArtifactPath
	if !strings.Contains(fb.gotReq.Prompt, want) {
		t.Errorf("plan prompt must instruct the ABSOLUTE artifact path %q so the agent writes where the bridge watches; a relative path lands in the REPL cwd → the cycle-210 artifact-timeout. Prompt:\n%s", want, fb.gotReq.Prompt)
	}
	if fb.gotReq.ArtifactPath != want {
		t.Errorf("bridge ArtifactPath=%q, want %q (prompt + watched path must agree)", fb.gotReq.ArtifactPath, want)
	}
}

// Proves the configured {cli,model} actually reach BridgeRequest.{CLI,Model}
// on a Plan launch, and that the uniform contract (artifact completion,
// routing-plan.json, injected persona) holds identically for non-claude CLIs
// — the any-CLI × any-model invariant.
func TestPhaseAdvisor_DispatchWiringFlowsToBridge(t *testing.T) {
	t.Parallel()
	plan := `[{"phase":"scout","run":true,"justification":"x"}]`
	cases := []struct{ cli, model string }{
		{"codex-tmux", "gpt-5.5"},   // openai family, deep model
		{"agy", "gemini-3.5-flash"}, // google family, headless
		{"claude-tmux", "opus"},     // anthropic default
	}
	for _, c := range cases {
		c := c
		t.Run(c.cli+"/"+c.model, func(t *testing.T) {
			t.Parallel()
			fb := &fakeBridge{stdout: plan}
			adv := NewPhaseAdvisor(fb,
				WithProposerCLI(c.cli),
				WithProposerModel(c.model),
				WithPersona("PERSONA_MARKER_42"),
			)
			if _, err := adv.Plan(baseRouteInput()); err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if fb.gotReq.CLI != c.cli {
				t.Errorf("BridgeRequest.CLI=%q, want %q (config must flow to the bridge)", fb.gotReq.CLI, c.cli)
			}
			if fb.gotReq.Model != c.model {
				t.Errorf("BridgeRequest.Model=%q, want %q", fb.gotReq.Model, c.model)
			}
			if fb.gotReq.Completion != "artifact" {
				t.Errorf("Completion=%q, want artifact for %s", fb.gotReq.Completion, c.cli)
			}
			if !strings.HasSuffix(fb.gotReq.ArtifactPath, "routing-plan.json") {
				t.Errorf("artifact=%q, want .../routing-plan.json for %s", fb.gotReq.ArtifactPath, c.cli)
			}
			if !strings.Contains(fb.gotReq.Prompt, "PERSONA_MARKER_42") {
				t.Errorf("persona missing from prompt for %s (persona path must hold for non-claude CLIs)", c.cli)
			}
			if fb.gotReq.Agent != "router" {
				t.Errorf("Agent=%q, want router", fb.gotReq.Agent)
			}
		})
	}
}

func TestPhaseAdvisor_PlanFailSafe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fb   *fakeBridge
		in   router.RouteInput
	}{
		{"bridge error", &fakeBridge{err: errors.New("boom")}, baseRouteInput()},
		{"no array", &fakeBridge{stdout: "I could not decide."}, baseRouteInput()},
		{"empty array", &fakeBridge{stdout: "[]"}, baseRouteInput()},
		{"malformed array", &fakeBridge{stdout: `[{"phase":}]`}, baseRouteInput()},
	}
	for _, c := range cases {
		if _, err := NewPhaseAdvisor(c.fb).Plan(c.in); err == nil {
			t.Errorf("%s: want error (so caller degrades to static), got nil", c.name)
		}
	}
	if _, err := NewPhaseAdvisor(nil).Plan(baseRouteInput()); err == nil {
		t.Error("nil bridge: want error")
	}
	noWs := baseRouteInput()
	noWs.Workspace = ""
	if _, err := NewPhaseAdvisor(&fakeBridge{stdout: "[]"}).Plan(noWs); err == nil {
		t.Error("empty workspace: want error")
	}
}
