package swarmplan

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhase_Identity(t *testing.T) {
	p := New(Config{})
	if got := p.PhaseName(); got != "swarm-plan" {
		t.Errorf("PhaseName = %q, want swarm-plan", got)
	}
	if got := p.AgentPromptName(); got != "evolve-swarm-planner" {
		t.Errorf("AgentPromptName = %q, want evolve-swarm-planner", got)
	}
	if got := p.ArtifactFilename(core.PhaseRequest{}); got != "swarm-plan.md" {
		t.Errorf("ArtifactFilename = %q, want swarm-plan.md", got)
	}
	if got := p.DefaultModel(); got != "opus" {
		t.Errorf("DefaultModel = %q, want opus", got)
	}
}

// ShouldSkip must skip by default (shadow): no EVOLVE_SWARM_PLANNER → opt-in
// phase is skipped, and the cycle proceeds to build unchanged.
func TestPhase_ShouldSkip_DefaultShadow(t *testing.T) {
	p := New(Config{})
	skip, verdict, next, _ := p.ShouldSkip(core.PhaseRequest{Env: map[string]string{}})
	if !skip {
		t.Fatal("swarm-plan must skip by default (shadow / opt-in)")
	}
	if verdict != core.VerdictSKIPPED {
		t.Errorf("verdict = %q, want SKIPPED", verdict)
	}
	if next != string(core.PhaseBuild) {
		t.Errorf("next = %q, want build", next)
	}
}

func TestPhase_ShouldSkip_EnabledRuns(t *testing.T) {
	p := New(Config{})
	skip, _, _, _ := p.ShouldSkip(core.PhaseRequest{Env: map[string]string{"EVOLVE_SWARM_PLANNER": "1"}})
	if skip {
		t.Error("swarm-plan must run when EVOLVE_SWARM_PLANNER=1")
	}
}

func TestPhase_Classify(t *testing.T) {
	p := New(Config{})
	if v, _, _ := p.Classify("", core.PhaseRequest{}, core.BridgeResponse{}); v != core.VerdictFAIL {
		t.Errorf("empty artifact: verdict = %q, want FAIL", v)
	}
	v, _, next := p.Classify("# Swarm Plan\n...", core.PhaseRequest{}, core.BridgeResponse{})
	if v != core.VerdictPASS {
		t.Errorf("non-empty artifact: verdict = %q, want PASS", v)
	}
	if next != string(core.PhaseBuild) {
		t.Errorf("next = %q, want build", next)
	}
}

// TestBaseRunner verifies that BaseRunner returns a non-nil runner.BaseRunner,
// confirming the factory wiring is correct.
func TestBaseRunner(t *testing.T) {
	p := New(Config{})
	br := p.BaseRunner()
	if br == nil {
		t.Fatal("BaseRunner must return a non-nil *runner.BaseRunner")
	}
}

// TestComposePrompt verifies that ComposePrompt injects the cycle context.
func TestComposePrompt(t *testing.T) {
	p := New(Config{})
	req := core.PhaseRequest{Cycle: 42, GoalHash: "abc123", ProjectRoot: "/proj", Workspace: "/ws"}
	got := p.ComposePrompt("body content", req)
	if !strings.Contains(got, "body content") {
		t.Error("ComposePrompt must preserve the body")
	}
	if !strings.Contains(got, "42") {
		t.Error("ComposePrompt must inject cycle number")
	}
	if !strings.Contains(got, "abc123") {
		t.Error("ComposePrompt must inject goal hash")
	}
}
