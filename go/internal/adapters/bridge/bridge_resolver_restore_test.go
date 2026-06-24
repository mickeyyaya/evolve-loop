package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestSetContractResolver_NilRestoresBuiltinOnly pins SetContractResolver's
// documented nil contract: after a catalog resolver has been wired (so a user
// phase resolves to a spec-derived Deliverable Contract), passing nil must
// RESTORE built-in-only resolution — the same back-compat posture a fresh
// Adapter has. The existing suite covers the two end states (catalog-resolver
// injects; default resolver does not) but nothing pins the TRANSITION back to
// the default, which is exactly the nil-guard branch in SetContractResolver.
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

	// Precondition: the wired catalog resolver injects the contract for "foo".
	if !strings.Contains(launch(), "Deliverable Contract") {
		t.Fatal("precondition: catalog resolver should inject the contract for user phase foo")
	}

	// Act: restore built-in-only resolution.
	a.SetContractResolver(nil)

	// Assert: "foo" is unknown to the built-in resolver, so the prompt now
	// passes through with no contract — the nil call genuinely restored the default.
	if got := launch(); strings.Contains(got, "Deliverable Contract") {
		t.Errorf("SetContractResolver(nil) must restore built-in-only resolution; user phase foo still got a contract; prompt=%q", got)
	}
}

// TestInjectContract_ZeroValueAdapter_DegradesToBuiltin pins the documented
// degradation of a zero-value Adapter (resolver field nil): injectContract must
// fall back to the built-in resolver rather than nil-panic. For a user phase the
// built-in resolver has no entry, so the prompt body passes through unchanged.
func TestInjectContract_ZeroValueAdapter_DegradesToBuiltin(t *testing.T) {
	var a Adapter // zero value: resolver is nil

	if got := a.injectContract("BODY", "foo", "/ws/foo-report.md"); got != "BODY" {
		t.Errorf("zero-value Adapter must degrade to built-in-only and pass an unregistered user phase through unchanged; got %q", got)
	}
}
