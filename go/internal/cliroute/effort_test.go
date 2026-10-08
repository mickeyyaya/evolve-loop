package cliroute_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCompile_AnEffortOnlyTierOrAgentRuleInheritsTheChain(t *testing.T) {
	block := policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Tiers:  map[string]policy.TierRule{"fast": {Effort: "low"}, "deep": {CLIs: []string{"claude"}, Effort: "high"}},
		Agents: map[string]policy.AgentRule{"scout": {Effort: "low"}},
	}
	if errs := errorFindings(compileSynthetic(t, block)); len(errs) > 0 {
		t.Fatalf("an effort-only rule is refused: %+v", errs)
	}
}

func TestCompile_RefusesABadEffortAndAnEmptyTierRule(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Tiers:  map[string]policy.TierRule{"deep": {Effort: "hihg"}, "top": {}, "galactic": {Effort: "low"}},
		Agents: map[string]policy.AgentRule{"scout": {Effort: "ultra"}, "nobody": {Effort: "low"}},
	})
	requireFinding(t, findings, "cli_routing.tiers.deep.effort", cliroute.SeverityError, "hihg", "low", "max")
	requireFinding(t, findings, "cli_routing.tiers.top", cliroute.SeverityError, "empty")
	requireFinding(t, findings, "cli_routing.tiers.galactic", cliroute.SeverityError, "galactic")
	requireFinding(t, findings, "cli_routing.agents.scout.effort", cliroute.SeverityError, "ultra")
	requireFinding(t, findings, "cli_routing.agents.nobody", cliroute.SeverityError, "profile")
}

func TestRouter_EffortNamesItsSource(t *testing.T) {
	block := policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Tiers:  map[string]policy.TierRule{"deep": {Effort: "high"}},
		Agents: map[string]policy.AgentRule{"scout": {Effort: "low"}, "auditor": {CLI: []string{"claude"}, Effort: "xhigh"}},
	}
	r := mustRouter(t, mustCompile(t, routingPolicy(block), syntheticCatalog(), syntheticProfiles(t)), cliroute.Host{LookPath: everyBinary()})
	cases := []struct{ agent, tier, want, source string }{
		{"scout", "balanced", "low", "cli_routing.agents.scout"},
		{"auditor", "deep", "xhigh", "cli_routing.agents.auditor"},
		{"builder", "deep", "high", "cli_routing.tiers.deep"},
		{"builder", "top", "medium", "default"},
	}
	for _, tc := range cases {
		if got, source := r.Effort(tc.agent, tc.tier); got != tc.want || source != tc.source {
			t.Errorf("Effort(%s, %s) = %q from %q, want %q from %q", tc.agent, tc.tier, got, source, tc.want, tc.source)
		}
	}
}

func TestCompile_AnEffortOnAPhaseAliasIsRefused(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Agents: map[string]policy.AgentRule{"audit": {CLI: []string{"claude"}, Effort: "high"}},
	})
	requireFinding(t, findings, "cli_routing.agents.audit.effort", cliroute.SeverityError, "auditor")
}
