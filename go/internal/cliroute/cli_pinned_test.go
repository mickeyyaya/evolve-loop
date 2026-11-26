package cliroute_test

import (
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func cloudBuilderProfile(t *testing.T, pinned string) profiles.Profile {
	t.Helper()
	body := []byte(`{"name":"cloud-builder","role":"cloud-builder","cli":"claude-cloud","cli_pinned":` + pinned + `,"model_tier_default":"balanced","model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"}}`)
	prof, err := profiles.NewFromFS(fstest.MapFS{"cloud-builder.json": &fstest.MapFile{Data: body}}).Get("cloud-builder")
	if err != nil {
		t.Fatal(err)
	}
	return prof
}

func cloudBuilderRouter(t *testing.T, block policy.CLIRouting, pinned string) *cliroute.Router {
	t.Helper()
	prof := cloudBuilderProfile(t, pinned)
	return reportRouter(t, routingPolicy(block), prof, false)
}

func cloudBuilderRequest(mutate func(*cliroute.Request)) cliroute.Request {
	req := cliroute.Request{Agent: "cloud-builder", DefaultModel: "balanced", NeedsTools: true}
	if mutate != nil {
		mutate(&req)
	}
	return req
}

var cloudDefault = policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}

func TestResolve_APinnedProfileKeepsItsCloudSeatFirstUnderADeclaredDefault(t *testing.T) {
	t.Parallel()
	r := cloudBuilderRouter(t, cloudDefault, "true")

	d := mustResolve(t, r, cloudBuilderRequest(nil))

	want := []string{"claude-cloud", "agy-tmux", "claude-tmux"}
	if !reflect.DeepEqual(d.Plan.Candidates, want) || d.Rule != "profile-pinned" {
		t.Fatalf("chain = %v rule = %q, want %v from rule profile-pinned (trace %q)", d.Plan.Candidates, d.Rule, want, d.Trace)
	}
	requireTrace(t, d, "profile cloud-builder pins claude-cloud")
}

func TestResolve_AnUnpinnedProfileStillFollowsTheDeclaredDefault(t *testing.T) {
	t.Parallel()
	r := cloudBuilderRouter(t, cloudDefault, "false")

	d := mustResolve(t, r, cloudBuilderRequest(nil))

	want := []string{"agy-tmux", "claude-tmux"}
	if !reflect.DeepEqual(d.Plan.Candidates, want) || d.Rule != "default" {
		t.Fatalf("chain = %v rule = %q, want %v from rule default", d.Plan.Candidates, d.Rule, want)
	}
}

func TestResolve_AnAgentsRuleBeatsTheProfilePin(t *testing.T) {
	t.Parallel()
	block := cloudDefault
	block.Agents = map[string]policy.AgentRule{"cloud-builder": {CLI: []string{"claude"}}}
	r := cloudBuilderRouter(t, block, "true")

	d := mustResolve(t, r, cloudBuilderRequest(nil))

	if d.Plan.Candidates[0] != "claude-tmux" || d.Rule != "agents:cloud-builder" {
		t.Fatalf("chain = %v rule = %q, want claude-tmux first from the operator's agents rule", d.Plan.Candidates, d.Rule)
	}
}

func TestResolve_APinnedProfileKeepsItsCloudSeatFirstAtTheDeepTier(t *testing.T) {
	t.Parallel()
	block := cloudDefault
	block.Tiers = map[string]policy.TierRule{"deep": {CLIs: []string{"claude", "agy"}, SharePct: 1}}
	r := cloudBuilderRouter(t, block, "true")

	for cycle := 1; cycle <= 20; cycle++ {
		d := mustResolve(t, r, cloudBuilderRequest(func(req *cliroute.Request) {
			req.Cycle = cycle
			req.Overlay = llmroute.Overlay{LadderTier: "deep"}
		}))

		if d.Plan.Candidates[0] != "claude-cloud" {
			t.Fatalf("cycle %d: chain %v trace %q; the pin keeps the cloud seat first at the deep tier", cycle, d.Plan.Candidates, d.Trace)
		}
		requireTrace(t, d, "skipped: profile cloud-builder pins claude")
	}
}

func TestResolve_APinnedProfileLeadsTheChainOfAWorkRule(t *testing.T) {
	t.Parallel()
	block := cloudDefault
	block.Work = map[string][]string{"build": {"agy"}}
	r := cloudBuilderRouter(t, block, "true")

	d := mustResolve(t, r, cloudBuilderRequest(func(req *cliroute.Request) { req.Phase = "build" }))

	if d.Plan.Candidates[0] != "claude-cloud" || d.Rule != "profile-pinned" {
		t.Fatalf("chain = %v rule = %q, want claude-cloud first from rule profile-pinned (trace %q)", d.Plan.Candidates, d.Rule, d.Trace)
	}
}

func TestResolve_AnAdvisorOrCallerCLICannotMoveAPinnedSeat(t *testing.T) {
	t.Parallel()
	r := cloudBuilderRouter(t, cloudDefault, "true")

	for name, mutate := range map[string]func(*cliroute.Request){
		"the caller cli":  func(req *cliroute.Request) { req.CallerCLI = "agy-tmux" },
		"the advisor cli": func(req *cliroute.Request) { req.Overlay.CLI = "agy" },
	} {
		t.Run(name, func(t *testing.T) {
			d := mustResolve(t, r, cloudBuilderRequest(mutate))

			if d.Plan.Candidates[0] != "claude-cloud" {
				t.Fatalf("chain %v trace %q, want the cloud seat first", d.Plan.Candidates, d.Trace)
			}
		})
	}
}

func TestResolve_TheTrackedPolicyAndCloudBuilderProfileStartOnTheCloudSeatThenFollowTheDefault(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	profs := profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles"))
	table := mustCompile(t, pol, realCatalog(t), profs)
	r := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})

	d := mustResolve(t, r, cliroute.Request{Agent: "cloud-builder", DefaultModel: "balanced", NeedsTools: true})

	want := []string{"claude-cloud"}
	for _, fam := range pol.CLIRouting.Default {
		want = append(want, llmroute.DefaultDriverForFamily(fam))
	}
	if !reflect.DeepEqual(d.Plan.Candidates, want) {
		t.Fatalf("chain = %v, want the cloud seat then the default order %v (trace %q)", d.Plan.Candidates, want, d.Trace)
	}
}
