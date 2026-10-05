package cliroute_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestResolve_AbsentTableMatchesLegacyGolden(t *testing.T) {
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	profs := profiles.NewFromDir(profileDir)
	cat := realCatalog(t)
	var runnerRecords, chainRecords []cliroutetest.Record
	for _, v := range cliroutetest.Variants() {
		root := v.ProjectRoot(t, cliroutetest.PhasesOf(agents))
		table, findings := cliroute.Compile(legacyPolicyFor(v, v.Policy(t, root)), cat, profs)
		if len(findings) != 0 {
			t.Fatalf("variant %s: the legacy projection has no findings, got %+v", v.Name, findings)
		}
		r := mustRouter(t, table, cliroute.Host{LookPath: v.LookPath, Discover: v.RawDiscover, Bench: realBench})
		for _, agent := range agents {
			phase := cliroutetest.PhaseOf(agent)
			runnerRecords = append(runnerRecords, legacyRecord(r, cliroutetest.ResolverRunner, v, agent, phase, cliroute.Request{
				Agent: agent, Phase: phase, ProjectRoot: root, DefaultModel: cliroutetest.DefaultModel,
				Env: v.EnvFor(agent), Overlay: v.Overlay,
			}))
			chainRecords = append(chainRecords, legacyRecord(r, cliroutetest.ResolverBridgechain, v, agent, "", cliroute.Request{
				Agent: agent, ProjectRoot: root, DefaultModel: cliroutetest.DefaultModel,
				Env: v.EnvFor(agent), CallerCLI: v.CallerCLI,
			}))
		}
	}
	cliroutetest.AssertGolden(t, append(runnerRecords, chainRecords...), false)
}

func legacyPolicyFor(v cliroutetest.Variant, loaded policy.Policy) policy.Policy {
	if v.Bypass {
		return policy.Policy{Workflow: loaded.Workflow}
	}
	return loaded
}

func legacyRecord(r *cliroute.Router, resolver string, v cliroutetest.Variant, agent, phase string, req cliroute.Request) cliroutetest.Record {
	d, err := r.Resolve(req)
	if err != nil {
		return cliroutetest.ErrorRecord(resolver, v.Name, agent, phase, err.Error())
	}
	return cliroutetest.PlanRecord(resolver, v.Name, agent, phase, d.Plan)
}

func TestResolve_ALegacyDecisionNamesItsRuleAndAllowedSet(t *testing.T) {
	p := policy.Policy{Pins: map[string]policy.Pin{"scout": {CLI: "claude"}}}
	table, _ := cliroute.Compile(p, syntheticCatalog(), syntheticProfiles(t))
	r := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})
	cases := []struct {
		name    string
		req     cliroute.Request
		rule    string
		allowed []string
	}{
		{"pin", cliroute.Request{Agent: "scout", Phase: "scout"}, "legacy:pin", nil},
		{"env", cliroute.Request{Agent: "memo", Phase: "memo", Env: map[string]string{"EVOLVE_MEMO_CLI": "claude-p"}}, "legacy:env", nil},
		{"caller", cliroute.Request{Agent: "memo", CallerCLI: "claude-p"}, "legacy:caller", nil},
		{"profile", cliroute.Request{Agent: "auditor", Phase: "audit"}, "legacy:profile", []string{"claude"}},
		{"default", cliroute.Request{Agent: "nobody"}, "legacy:default", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := mustResolve(t, r, tc.req)
			if d.Rule != tc.rule || !reflect.DeepEqual(d.Allowed, tc.allowed) {
				t.Fatalf("rule=%s allowed=%v, want %s %v", d.Rule, d.Allowed, tc.rule, tc.allowed)
			}
		})
	}
}

func TestResolve_ALegacyPinOutsideTheProfileIsTheRunnersError(t *testing.T) {
	p := policy.Policy{Pins: map[string]policy.Pin{"audit": {CLI: "agy"}}}
	table, _ := cliroute.Compile(p, syntheticCatalog(), syntheticProfiles(t))
	_, err := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()}).Resolve(cliroute.Request{Agent: "auditor", Phase: "audit"})
	want := `policy: pin for phase "audit": cli "agy" not in allowed_clis [claude]`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want the runner's own refusal %q", err, want)
	}
}

func TestResolve_TheLegacyTailHonoursTheFamilyBanAndTheSwitch(t *testing.T) {
	discover := func() []string { return []string{"claude-tmux", "agy-tmux"} }
	off, none := false, []string{}
	cases := []struct {
		name     string
		workflow *policy.WorkflowPolicy
		want     []string
	}{
		{"compiled default bans agy", nil, []string{"codex-tmux", "claude-tmux"}},
		{"an empty ban admits agy", &policy.WorkflowPolicy{UniversalFallbackExclude: none}, []string{"codex-tmux", "claude-tmux", "agy-tmux"}},
		{"the switch off appends nothing", &policy.WorkflowPolicy{UniversalFallback: &off, UniversalFallbackExclude: none}, []string{"codex-tmux", "claude-tmux"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table, _ := cliroute.Compile(policy.Policy{Workflow: tc.workflow}, syntheticCatalog(), syntheticProfiles(t))
			d := mustResolve(t, mustRouter(t, table, cliroute.Host{LookPath: everyBinary(), Discover: discover}), cliroute.Request{Agent: "scout", Phase: "scout"})
			if !reflect.DeepEqual(d.Plan.Candidates, tc.want) {
				t.Fatalf("chain = %v, want %v", d.Plan.Candidates, tc.want)
			}
		})
	}
}

func TestResolve_ALegacyProfileThatFailsToLoadResolvesAsNoProfile(t *testing.T) {
	broken := profiles.NewFromFS(fstest.MapFS{
		"x.json": {Data: []byte(`{"name":"x","cli":"agy-tmux","allowed_tools":["$include_policy:missing"]}`)},
	})
	table, findings := cliroute.Compile(policy.Policy{}, syntheticCatalog(), broken)
	if len(findings) != 0 {
		t.Fatalf("the legacy projection keeps today's silent nil profile: %+v", findings)
	}
	d := mustResolve(t, mustRouter(t, table, cliroute.Host{LookPath: everyBinary()}), cliroute.Request{Agent: "x"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux"}) || d.Rule != "legacy:default" {
		t.Fatalf("an unloadable profile resolves as the bridge chain does today, with no profile: %+v", d)
	}
}
