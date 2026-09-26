package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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

// build has no Verdicts, so "evolve-verdict" reaches its prompt only through the failure instruction.
func TestLaunch_PhaseIOFailureInstruction_GatedByStage(t *testing.T) {
	launchBuild := func(stage config.Stage) string {
		fe := &fakeEngine{}
		a := withEngine(fe)
		a.SetPhaseIOStage(stage)
		if _, err := a.Launch(context.Background(), core.BridgeRequest{
			CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
			Workspace: t.TempDir(), ArtifactPath: "/ws/build-report.md", Agent: "build",
		}); err != nil {
			t.Fatalf("Launch: %v", err)
		}
		return fe.gotReq.Prompt
	}

	off := launchBuild(config.StageOff)
	if strings.Contains(off, "evolve-verdict") {
		t.Errorf("PhaseIO=off: build prompt must NOT carry the failure-sentinel instruction; got:\n%s", off)
	}
	if launchBuild(config.StageShadow) != off {
		t.Error("PhaseIO=shadow must be byte-identical to off for the contract block")
	}
	for _, s := range []config.Stage{config.StageAdvisory, config.StageEnforce} {
		if on := launchBuild(s); !strings.Contains(on, "evolve-verdict") {
			t.Errorf("PhaseIO=%s: build prompt must carry the failure-sentinel instruction; got:\n%s", s, on)
		}
	}
}

func TestLaunch_DefaultResolver_NoContractForUserPhase(t *testing.T) {
	fe := &fakeEngine{}
	a := withEngine(fe)
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

func TestSetRecoveryStage_WiresField(t *testing.T) {
	a := New()
	for _, stage := range []string{"", "shadow", "enforce", "off"} {
		a.SetRecoveryStage(stage)
		if a.recoveryStage != stage {
			t.Errorf("SetRecoveryStage(%q) → a.recoveryStage = %q, want %q", stage, a.recoveryStage, stage)
		}
	}
}
