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
