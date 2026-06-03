package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// fakeBridge records the request and returns canned output.
type fakeBridge struct {
	stdout string
	err    error
	gotReq BridgeRequest
	calls  int
}

func (f *fakeBridge) Launch(_ context.Context, req BridgeRequest) (BridgeResponse, error) {
	f.calls++
	f.gotReq = req
	if f.err != nil {
		return BridgeResponse{}, f.err
	}
	return BridgeResponse{Stdout: f.stdout, ExitCode: 0}, nil
}
func (f *fakeBridge) Probe(_ context.Context) (BridgeProbe, error) { return BridgeProbe{}, nil }

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
	fb := &fakeBridge{stdout: `{"next_phase":"tester","insert_phases":["tester"],"justification":"acs red"}`}
	p := NewPhaseAdvisor(fb)
	prop, err := p.Propose(baseRouteInput())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if prop.NextPhase != "tester" || len(prop.InsertPhases) != 1 || prop.InsertPhases[0] != "tester" {
		t.Errorf("proposal=%+v, want next=tester insert=[tester]", prop)
	}
	// The proposer wires the router profile path + workspace artifact.
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

func TestPhaseAdvisor_TolerantOfFenceAndProse(t *testing.T) {
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
	// nil bridge + empty workspace also error without panicking.
	if _, err := NewPhaseAdvisor(nil).Propose(baseRouteInput()); err == nil {
		t.Error("nil bridge: want error")
	}
	noWs := baseRouteInput()
	noWs.Workspace = ""
	if _, err := NewPhaseAdvisor(&fakeBridge{stdout: "{}"}).Propose(noWs); err == nil {
		t.Error("empty workspace: want error")
	}
}

// Integration: LLMProposal defers to the proposer, but router.Route CLAMPS an
// illegal proposal to the kernel's legal next — proving "model proposes, kernel
// disposes". Proposer says ship (illegal from build); kernel forces audit.
func TestPhaseAdvisor_ProposalIsClampedByKernel(t *testing.T) {
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
	in.BudgetRemaining = 100

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

// TestPhaseAdvisor_PlanParsesArray covers Plan + parsePhasePlan: the whole-cycle
// JSON array (run true+false mix), fence/prose tolerance, and that the plan path
// wires phase-plan.json (distinct from Propose's routing-proposal.json).
func TestPhaseAdvisor_PlanParsesArray(t *testing.T) {
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
			// The advisor's RAW plan artifact is routing-plan.json, distinct from
			// the orchestrator's clamped phase-plan.json (both survive for
			// forensics). The Plan path now uses the UNIFORM artifact-completion
			// contract (the brain WRITES routing-plan.json; the bridge reads it
			// back) — same as every phase agent, replacing the brittle stdout scrape.
			if !strings.HasSuffix(fb.gotReq.ArtifactPath, "routing-plan.json") {
				t.Errorf("artifact=%q, want .../routing-plan.json", fb.gotReq.ArtifactPath)
			}
			if fb.gotReq.Completion != "artifact" {
				t.Errorf("Completion=%q, want artifact", fb.gotReq.Completion)
			}
		})
	}
}

// TestPhaseAdvisor_PersonaComposition proves the Plan prompt is composed the
// uniform way: the injected persona (agents/evolve-router.md body) followed by
// the dynamic per-cycle context — and falls back to the inline framing when no
// persona is injected.
func TestPhaseAdvisor_PersonaComposition(t *testing.T) {
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
		adv := NewPhaseAdvisor(fb) // no persona injected
		if _, err := adv.Plan(router.RouteInput{Workspace: "/tmp/x", Cycle: 7}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(fb.gotReq.Prompt, "PHASE ADVISOR") {
			t.Error("fallback prompt must use the legacy inline framing")
		}
	})
}

// TestPhaseAdvisor_DispatchWiringFlowsToBridge proves the configured {cli,model}
// actually REACH BridgeRequest.{CLI,Model} on a Plan launch — and that the
// uniform contract (artifact completion + routing-plan.json + injected persona)
// holds identically for non-claude CLIs. This is the heart of the any-CLI ×
// any-model invariant: the brain is dispatched to whatever the composition root
// resolved, not a hardcoded claude/opus. (Regression guard: TestPhaseAdvisorOptions
// pins the struct fields; this pins the field→bridge flow that advisorLaunch owns.)
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
			// The uniform contract is CLI-agnostic: artifact completion, the
			// routing-plan.json deliverable, and the injected persona hold for
			// every CLI, not just claude.
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

// TestPhaseAdvisor_PlanFailSafe proves every malformed/failed plan returns an
// error so the caller degrades to the deterministic static path (fail to floor).
func TestPhaseAdvisor_PlanFailSafe(t *testing.T) {
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
	// nil bridge + empty workspace also error without panicking.
	if _, err := NewPhaseAdvisor(nil).Plan(baseRouteInput()); err == nil {
		t.Error("nil bridge: want error")
	}
	noWs := baseRouteInput()
	noWs.Workspace = ""
	if _, err := NewPhaseAdvisor(&fakeBridge{stdout: "[]"}).Plan(noWs); err == nil {
		t.Error("empty workspace: want error")
	}
}

// TestBuildPlanPrompt_WholeCycleArray proves the plan prompt shares the routing
// context (rubric) with buildRoutingPrompt but asks for the whole-cycle ARRAY
// shape, not the per-transition object — the two cadences diverge correctly.
func TestBuildPlanPrompt_WholeCycleArray(t *testing.T) {
	t.Parallel()
	in := router.RouteInput{
		Current:   "start",
		Cycle:     3,
		Completed: []string{},
		Signals:   router.RoutingSignals{Scout: router.ScoutSignals{CycleSizeEstimate: "medium", Present: true}},
	}
	got := buildPlanPrompt(in)
	// shared context (rubric line from writeRoutingContext)
	if !strings.Contains(got, "skip scout (work already queued)") {
		t.Errorf("plan prompt missing shared rubric:\n%s", got)
	}
	// whole-cycle array spec, NOT the per-transition object spec
	if !strings.Contains(got, `[{"phase":"<phase>","run":true`) {
		t.Errorf("plan prompt missing array JSON spec:\n%s", got)
	}
	if strings.Contains(got, `"next_phase":"<phase>"`) {
		t.Errorf("plan prompt should not carry the per-transition object spec:\n%s", got)
	}
}
