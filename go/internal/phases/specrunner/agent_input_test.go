package specrunner

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func TestComposePrompt_AgentInputKeyReachesASpecWithNoPromptContext(t *testing.T) {
	h := hooks{spec: phasespec.PhaseSpec{Name: "risk-register"}}
	diff := "diff --git a/x.go b/x.go\n+func X() {}"

	got := h.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{AgentInputKey: diff}})

	if !strings.Contains(got, "\n\n## Agent Input\n") {
		t.Fatalf("prompt has no Agent Input section:\n%s", got)
	}
	if !strings.Contains(got, diff) {
		t.Errorf("prompt does not carry the input verbatim:\n%s", got)
	}
	if !strings.Contains(got, "replaces the cycle artifacts") {
		t.Errorf("prompt does not tell the agent that the input replaces the cycle artifacts:\n%s", got)
	}
	if strings.Index(got, "## Agent Input") < strings.Index(got, "## Cycle Context") {
		t.Errorf("Agent Input must follow the cycle context:\n%s", got)
	}
}

func TestComposePrompt_NoAgentInputSectionWithoutTheKey(t *testing.T) {
	h := hooks{spec: phasespec.PhaseSpec{Name: "risk-register", PromptContext: []string{"goal"}}}

	got := h.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{"goal": "g"}})

	if strings.Contains(got, "Agent Input") {
		t.Errorf("a cycle prompt must not carry an Agent Input section:\n%s", got)
	}
}

func TestRun_AgentInputReachesTheDispatchedPrompt(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "adversarial-review", Classify: &phasespec.ClassifyRules{VerdictOnPass: core.VerdictPASS}}
	fb := &fakeBridge{writeArtifact: "# ok\n"}
	phase := New(spec, Config{Bridge: fb, Prompts: prompts.NewFromFS(fstest.MapFS{}), PromptBody: "PERSONA"})

	_, err := phase.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
		Context: map[string]string{AgentInputKey: "+added line"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(fb.gotReq.Prompt, "## Agent Input") || !strings.Contains(fb.gotReq.Prompt, "+added line") {
		t.Errorf("dispatched prompt lacks the agent input:\n%s", fb.gotReq.Prompt)
	}
}

func TestArtifactName(t *testing.T) {
	cases := []struct {
		name string
		spec phasespec.PhaseSpec
		want string
	}{
		{"declared output", phasespec.PhaseSpec{Name: "adversarial-review", Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/adversarial-review-report.md"}}}, "adversarial-review-report.md"},
		{"no output", phasespec.PhaseSpec{Name: "risk-register"}, "risk-register-report.md"},
		{"empty first output", phasespec.PhaseSpec{Name: "smell-scan", Outputs: phasespec.IO{Files: []string{""}}}, "smell-scan-report.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ArtifactName(c.spec); got != c.want {
				t.Errorf("ArtifactName = %q, want %q", got, c.want)
			}
			if got := (hooks{spec: c.spec}).ArtifactFilename(core.PhaseRequest{}); got != c.want {
				t.Errorf("hooks.ArtifactFilename = %q, want %q", got, c.want)
			}
		})
	}
}
