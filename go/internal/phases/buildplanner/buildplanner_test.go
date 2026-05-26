package buildplanner

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestShouldSkip_ShadowMode(t *testing.T) {
	p := New(Config{})
	req := core.PhaseRequest{} // Env nil → EVOLVE_BUILD_PLANNER unset
	skip, verdict, next, diags := p.ShouldSkip(req)
	if !skip {
		t.Error("want skip=true when EVOLVE_BUILD_PLANNER unset (shadow mode)")
	}
	if verdict != core.VerdictSKIPPED {
		t.Errorf("verdict=%q, want SKIPPED", verdict)
	}
	if next != string(core.PhaseBuild) {
		t.Errorf("next=%q, want build", next)
	}
	if len(diags) != 0 {
		t.Errorf("diags=%v, want none", diags)
	}
}

func TestShouldSkip_AdvisoryMode(t *testing.T) {
	p := New(Config{})
	req := core.PhaseRequest{
		Env: map[string]string{"EVOLVE_BUILD_PLANNER": "1"},
	}
	skip, verdict, next, diags := p.ShouldSkip(req)
	if skip {
		t.Error("want skip=false when EVOLVE_BUILD_PLANNER=1 (advisory mode)")
	}
	if verdict != "" {
		t.Errorf("verdict=%q, want empty on non-skip", verdict)
	}
	if next != "" {
		t.Errorf("next=%q, want empty on non-skip", next)
	}
	if len(diags) != 0 {
		t.Errorf("diags=%v, want none", diags)
	}
}

func TestClassify_EmptyArtifact(t *testing.T) {
	p := New(Config{})
	verdict, diags, next := p.Classify("", core.PhaseRequest{}, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL for empty artifact", verdict)
	}
	if len(diags) == 0 {
		t.Error("want at least one diagnostic for empty artifact")
	}
	if next != string(core.PhaseBuild) {
		t.Errorf("next=%q, want build", next)
	}
}

func TestClassify_ValidArtifact(t *testing.T) {
	p := New(Config{})
	artifact := "## Implementation Steps\n\n1. Refactor the planner\n\n## Files to Modify\n- go/internal/phases/build/build.go\n"
	verdict, diags, next := p.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS for non-empty artifact", verdict)
	}
	if len(diags) != 0 {
		t.Errorf("diags=%v, want none for valid artifact", diags)
	}
	if next != string(core.PhaseBuild) {
		t.Errorf("next=%q, want build", next)
	}
}

func TestComposePrompt_ContextBlock(t *testing.T) {
	p := New(Config{})
	req := core.PhaseRequest{
		Cycle:       42,
		GoalHash:    "abc123deadbeef",
		ProjectRoot: "/proj/root",
		Workspace:   "/ws/cycle-42",
	}
	result := p.ComposePrompt("agent body text", req)
	for _, want := range []string{"42", "abc123deadbeef", "/proj/root", "/ws/cycle-42"} {
		if !strings.Contains(result, want) {
			t.Errorf("ComposePrompt missing %q; got:\n%s", want, result)
		}
	}
	if !strings.Contains(result, "agent body text") {
		t.Error("ComposePrompt must include original body")
	}
}

func TestPhaseMetadata(t *testing.T) {
	p := New(Config{})
	if got := p.PhaseName(); got != string(core.PhaseBuildPlanner) {
		t.Errorf("PhaseName=%q, want %q", got, string(core.PhaseBuildPlanner))
	}
	if got := p.AgentPromptName(); got != "evolve-build-planner" {
		t.Errorf("AgentPromptName=%q, want evolve-build-planner", got)
	}
	if got := p.DefaultModel(); got != "opus" {
		t.Errorf("DefaultModel=%q, want opus (anti-cooperative-bias requirement)", got)
	}
	if got := p.ArtifactFilename(core.PhaseRequest{}); got != "build-plan.md" {
		t.Errorf("ArtifactFilename=%q, want build-plan.md", got)
	}
}
