package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestSetContractResolver_NilRestoresBuiltinOnly(t *testing.T) {
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

	launch := func() string {
		fe.gotReq = core.BridgeRequest{}
		if _, err := a.Launch(context.Background(), core.BridgeRequest{
			CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
			Workspace: t.TempDir(), ArtifactPath: "/ws/foo-report.md", Agent: "foo",
		}); err != nil {
			t.Fatalf("Launch: %v", err)
		}
		return fe.gotReq.Prompt
	}

	if !strings.Contains(launch(), "Deliverable Contract") {
		t.Fatal("precondition: catalog resolver should inject the contract for user phase foo")
	}

	a.SetContractResolver(nil)

	if got := launch(); strings.Contains(got, "Deliverable Contract") {
		t.Errorf("SetContractResolver(nil) must restore built-in-only resolution; user phase foo still got a contract; prompt=%q", got)
	}
}

func TestInjectContract_ZeroValueAdapter_DegradesToBuiltin(t *testing.T) {
	var a Adapter

	got := a.injectContract("BODY", "foo", "/ws/foo-report.md", "/ws")
	if !strings.HasPrefix(got, "BODY") || !strings.Contains(got, "/ws/foo-report.md") {
		t.Errorf("unregistered phase with an artifact must keep body-first and disclose the path; got %q", got)
	}
	if got := a.injectContract("BODY", "foo", "", ""); got != "BODY" {
		t.Errorf("no artifact path ⇒ unchanged pass-through; got %q", got)
	}
}
