package cliroute_test

import (
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var floorAgents = []string{"auditor", "adversarial-review", "tdd-engineer", "spec-verifier", "spec-verify", "code-reviewer"}

func trackedPolicyWithRouting(t *testing.T, clis []string, floors policy.Floors) policy.Policy {
	t.Helper()
	p, err := policy.Load(filepath.Join(cliroutetest.RepoRoot(t), ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load the tracked policy: %v", err)
	}
	p.CLIRouting = &policy.CLIRouting{CLIs: clis, Default: clis, Floors: floors}
	return p
}

func trackedProfileSource(t *testing.T) cliroute.ProfileSource {
	t.Helper()
	return profiles.NewFromDir(filepath.Join(cliroutetest.RepoRoot(t), ".evolve", "profiles"))
}

func TestCompile_AnAgyOnlyTableRoutesTheSixFloorAgents(t *testing.T) {
	pol := trackedPolicyWithRouting(t, []string{"agy"}, policy.Floors{})
	_, findings := cliroute.Compile(pol, realCatalog(t), trackedProfileSource(t))
	var errKeys []string
	for _, f := range errorFindings(findings) {
		errKeys = append(errKeys, f.Key)
	}
	for _, agent := range floorAgents {
		if slices.Contains(errKeys, "agent."+agent) {
			t.Errorf("agent.%s is still refused over an agy-only table with floors {}", agent)
		}
	}
	if !slices.Contains(errKeys, "agent.router") {
		t.Errorf("the router ceiling must still refuse an agy-only table: error keys %v", errKeys)
	}
}

func TestCompile_AFloorAgentFollowsTheFloorWhenFloorsAreRestored(t *testing.T) {
	pol := trackedPolicyWithRouting(t, []string{"agy", "claude"}, policy.DefaultFloors())
	table, findings := cliroute.Compile(pol, realCatalog(t), trackedProfileSource(t))
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("compile reported errors: %+v", errs)
	}
	router := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})
	for _, agent := range floorAgents {
		d := mustResolve(t, router, cliroute.Request{Agent: agent})
		for _, cli := range d.Plan.Candidates {
			if llmroute.Family(cli) != "claude" {
				t.Errorf("%s: candidate %s leaves the claude floor: %v", agent, cli, d.Plan.Candidates)
			}
		}
	}
}

func TestResolve_ALegacyPinOutsideTheFloorIsRefusedWithoutAProfileCeiling(t *testing.T) {
	fsys := fstest.MapFS{"auditor.json": &fstest.MapFile{Data: []byte(`{"name":"auditor","cli":"claude-tmux","model_tier_default":"deep"}`)}}
	p := policy.Policy{Pins: map[string]policy.Pin{"audit": {CLI: "agy"}}}
	table, _ := cliroute.Compile(p, syntheticCatalog(), profiles.NewFromFS(fsys))
	_, err := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()}).Resolve(cliroute.Request{Agent: "auditor", Phase: "audit"})
	want := `policy: pin for phase "audit": cli "agy" not in allowed_clis [claude]`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want the floor to refuse the pin: %q", err, want)
	}
}
