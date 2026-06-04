package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestLaunch_InjectsContract_ForUserPhase proves a config-only phase gets the
// exact-output-path footer + required-section block injected — closing the
// ADR-0034 "agent infers/misses its output path" failure class for user/minted
// phases, with zero Go change to add the phase.
func TestLaunch_InjectsContract_ForUserPhase(t *testing.T) {
	fe := &fakeEngine{}
	a := withEngine(fe)

	foo := phasespec.PhaseSpec{
		Name:     "foo",
		Role:     "evaluate",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
		Outputs:  phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/foo-report.md"}},
	}
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{foo})
	a.SetContractResolver(phasecontract.NewCatalogResolver(cat.Get))

	_, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "PERSONA-BODY",
		Workspace: t.TempDir(), ArtifactPath: "/ws/foo-report.md", Agent: "foo",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt
	if !strings.Contains(got, "Deliverable Contract") {
		t.Error("expected the Deliverable Contract block to be injected for the user phase")
	}
	if !strings.Contains(got, "/ws/foo-report.md") {
		t.Errorf("expected the exact artifact path footer; prompt=%q", got)
	}
	if !strings.Contains(got, "## Findings") {
		t.Errorf("expected the canonical required-section heading injected; prompt=%q", got)
	}
}

// TestLaunch_DefaultResolver_NoContractForUserPhase confirms that WITHOUT a
// catalog resolver, a user phase still passes through unchanged (back-compat:
// the default Adapter resolves built-ins only).
func TestLaunch_DefaultResolver_NoContractForUserPhase(t *testing.T) {
	fe := &fakeEngine{}
	a := withEngine(fe) // no SetContractResolver → BuiltinResolver default
	_, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "foo",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if strings.Contains(fe.gotReq.Prompt, "Deliverable Contract") {
		t.Error("default resolver must not inject a contract for an unregistered user phase")
	}
}
