package main

import (
	"flag"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
)

var updateAdvisorGolden = flag.Bool("update", false, "rewrite the legacy advisor-dispatch golden")

type advisorBenchSet struct {
	name     string
	families map[string]bool
}

var advisorBenchSets = []advisorBenchSet{
	{name: "unbenched", families: map[string]bool{}},
	{name: "agy_benched", families: map[string]bool{"agy": true}},
	{name: "agy_claude_benched", families: map[string]bool{"agy": true, "claude": true}},
	{name: "claude_benched", families: map[string]bool{"claude": true}},
	{name: "agy_claude_target_benched", families: map[string]bool{"agy-claude": true}},
}

var advisorDecisions = []struct {
	name string
	dt   routerDecisionType
}{
	{"plan", decisionPlan},
	{"propose", decisionPropose},
}

func TestLegacyAdvisorDispatch_MatchesTheGolden(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	t.Setenv("EVOLVE_ROUTER_CLI", "")
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	var records []cliroutetest.Record
	for _, v := range cliroutetest.Variants() {
		for key, val := range v.EnvFor("router") {
			t.Setenv(key, val)
		}
		root := v.ProjectRoot(t, nil)
		pol := v.Policy(t, root)
		for _, bench := range advisorBenchSets {
			for _, d := range advisorDecisions {
				cli, model, ok := legacyAdvisorDispatch(t, filepath.Dir(profileDir), root, pol, d.dt, bench.families)
				records = append(records, cliroutetest.Record{
					Resolver: "advisor", Variant: v.Name + "/" + bench.name, Agent: "router", Phase: d.name,
					Candidates: []string{cli}, Model: model, Healthy: &ok,
				})
			}
		}
		for key := range v.EnvFor("router") {
			t.Setenv(key, "")
		}
	}
	cliroutetest.AssertGoldenAt(t, cliroutetest.AdvisorGoldenPath(t), records, *updateAdvisorGolden)
}
