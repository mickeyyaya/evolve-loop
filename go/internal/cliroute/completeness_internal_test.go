package cliroute

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func everyBinaryInstalled(bin string) (string, error) { return "/fake/bin/" + bin, nil }

func reviewerProbeBlock() policy.CLIRouting {
	return policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy"},
		Work: map[string][]string{"plan": {"claude"}, "build": {"claude"}, "evaluate": {"claude"}, "control": {"claude"}},
	}
}

func TestCompile_EveryRefusalResolveCanMakeIsACompileFinding(t *testing.T) {
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	cat, _, _, err := phasespec.MergedCatalog(cliroutetest.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	blocks := map[string]policy.CLIRouting{
		"reviewer probe":  reviewerProbeBlock(),
		"operator":        {CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}, Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}, "top": {CLIs: []string{"claude"}}}},
		"evaluate on agy": {CLIs: []string{"agy", "claude", "codex"}, Default: []string{"codex", "claude"}, Work: map[string][]string{"evaluate": {"agy"}}},
		"agy-only, stop":  {CLIs: []string{"agy", "claude"}, Default: []string{"agy"}, Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}, "top": {CLIs: []string{"claude"}}}, AfterChain: "stop"},
	}
	for name, block := range blocks {
		table, findings := Compile(policy.Policy{CLIRouting: &block}, cat, profiles.NewFromDir(profileDir))
		r := routerOf(table, Host{LookPath: everyBinaryInstalled})
		for _, agent := range agents {
			for _, phase := range []string{cliroutetest.PhaseOf(agent), ""} {
				_, err := r.Resolve(Request{Agent: agent, Phase: phase, DefaultModel: "balanced"})
				if err != nil && !reportsAgent(findings, table, agent) {
					t.Errorf("%s: Resolve(%s, phase %q) refuses (%v) with no keyed Compile finding", name, agent, phase, err)
				}
			}
		}
	}
}

func reportsAgent(findings []Finding, t Table, agent string) bool {
	keys := []string{"agent." + agent, "agent." + agent + ".tier_ceiling", "profiles." + agent}
	if key, assigned := t.agentKeys[agent]; assigned {
		keys = append(keys, "cli_routing.agents."+key)
	}
	for _, f := range findings {
		if f.Severity == SeverityError && slices.Contains(keys, f.Key) {
			return true
		}
	}
	return false
}

func TestCompile_ReportsTheReviewersTierCeilingProbe(t *testing.T) {
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	cat, _, _, err := phasespec.MergedCatalog(cliroutetest.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy"}, Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}, "top": {CLIs: []string{"claude"}}}, AfterChain: "stop"}
	table, findings := Compile(policy.Policy{CLIRouting: &block}, cat, profiles.NewFromDir(profileDir))
	if !reportsAgent(findings, table, "intent") {
		t.Fatalf("intent runs only at deep, where the agy-only chain has nothing to launch: %+v", findings)
	}
	r := routerOf(table, Host{LookPath: everyBinaryInstalled})
	_, resolveErr := r.Resolve(Request{Agent: "intent", Phase: "intent", DefaultModel: "balanced"})
	var ceilingErr *CeilingError
	if !errors.As(resolveErr, &ceilingErr) {
		t.Fatalf("Resolve refuses intent with a typed CeilingError, got %v", resolveErr)
	}
}

func TestReportsAgent_MatchesKeysNeverMessages(t *testing.T) {
	findings := []Finding{{Severity: SeverityError, Key: "agent.spec-verifier", Message: "rule default leaves no allowed CLI for spec-verify"}}
	if reportsAgent(findings, Table{}, "spec-verify") {
		t.Fatal("a finding about spec-verifier must not count for spec-verify")
	}
	if !reportsAgent(findings, Table{}, "spec-verifier") {
		t.Fatal("the keyed finding counts for its own agent")
	}
	if !reportsAgent([]Finding{{Severity: SeverityError, Key: "cli_routing.agents.audit"}}, Table{agentKeys: map[string]string{"auditor": "audit"}}, "auditor") {
		t.Fatal("the agents-leak key counts through the key the operator wrote")
	}
}

func TestResolve_APhaselessLaunchOfASingleRoleAgentGetsItsPhasesChain(t *testing.T) {
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	cat, _, _, err := phasespec.MergedCatalog(cliroutetest.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	block := reviewerProbeBlock()
	table, _ := Compile(policy.Policy{CLIRouting: &block}, cat, profiles.NewFromDir(profileDir))
	r := routerOf(table, Host{LookPath: everyBinaryInstalled})
	for _, agent := range []string{"tester", "retrospective"} {
		phased, errP := r.Resolve(Request{Agent: agent, Phase: cliroutetest.PhaseOf(agent), DefaultModel: "balanced"})
		phaseless, errN := r.Resolve(Request{Agent: agent, DefaultModel: "balanced"})
		if errP != nil || errN != nil || phased.Rule != phaseless.Rule || !reflect.DeepEqual(phased.Plan.Candidates, phaseless.Plan.Candidates) {
			t.Errorf("%s: phased %s %v (%v) vs phase-less %s %v (%v)", agent, phased.Rule, phased.Plan.Candidates, errP, phaseless.Rule, phaseless.Plan.Candidates, errN)
		}
	}
}

func TestResolve_ADeclaredProfileThatFailsToLoadIsRefused(t *testing.T) {
	broken := profiles.NewFromFS(fstest.MapFS{
		"x.json": {Data: []byte(`{"name":"x","cli":"agy-tmux","allowed_tools":["$include_policy:missing"]}`)},
	})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	table, findings := Compile(policy.Policy{CLIRouting: &block}, nil, broken)
	if len(findings) == 0 {
		t.Fatal("Compile reports the profile that does not load")
	}
	r := routerOf(table, Host{LookPath: everyBinaryInstalled})
	if _, err := r.Resolve(Request{Agent: "x"}); err == nil || !strings.Contains(err.Error(), "does not load") {
		t.Fatalf("a declared table never widens an unloadable profile to every CLI: %v", err)
	}
}

func TestApplyTierCeiling_APlanWithNoTierChainIsCheckedAtItsModel(t *testing.T) {
	r := resolver{table: Table{tiers: map[string][]string{"deep": {"claude"}}}}
	res := resolution{plan: llmroute.Plan{Candidates: []string{"agy-tmux"}, Model: "balanced"}}
	if _, err := applyTierCeiling(r, res); err != nil {
		t.Fatalf("an empty Tiers walks at the model, which the ceiling permits: %v", err)
	}
	res.plan.Model = "deep"
	if _, err := applyTierCeiling(r, res); err == nil {
		t.Fatal("an empty Tiers walks at the model; a deep model on agy has nothing to run")
	}
}

func routerOf(table Table, host Host) *Router {
	r := &Router{host: host}
	r.tables.Store(&tablePair{declared: table, bypass: table})
	return r
}
