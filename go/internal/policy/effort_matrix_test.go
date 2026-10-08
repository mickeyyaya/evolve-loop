package policy_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func checkedInPolicy(t *testing.T) policy.Policy {
	t.Helper()
	pol, err := policy.Load("../../../.evolve/policy.json")
	if err != nil {
		t.Fatalf("the checked-in policy must load: %v", err)
	}
	return pol
}

func TestTheCheckedInPolicy_EffortMatrix(t *testing.T) {
	table := checkedInPolicy(t).Efforts()
	cases := []struct{ agent, tier, want string }{
		{"scout", "balanced", "low"},
		{"triage", "balanced", "low"},
		{"reflector", "balanced", "low"},
		{"builder", "balanced", "medium"},
		{"builder", "deep", "medium"},
		{"tdd-engineer", "deep", "medium"},
		{"router", "deep", "medium"},
		{"auditor", "deep", "medium"},
		{"adversarial-review", "deep", "medium"},
		{"code-reviewer", "deep", "medium"},
		{"retrospective", "top", "medium"},
		{"memo", "fast", "low"},
	}
	for _, tc := range cases {
		if got, source := table.Resolve(tc.tier, tc.agent); got != tc.want {
			t.Errorf("%s at %s = %q from %q, want %q (the committed effort matrix)", tc.agent, tc.tier, got, source, tc.want)
		}
	}
}

func TestTheCheckedInPolicy_PinsNoEffortAboveMedium(t *testing.T) {
	block := checkedInPolicy(t).CLIRouting
	if block == nil {
		t.Fatal("the checked-in policy has no cli_routing block")
	}
	starting := []string{"low", "medium"}
	for tier, rule := range block.Tiers {
		if rule.Effort != "" && !slices.Contains(starting, rule.Effort) {
			t.Errorf("tiers.%s.effort = %q, want medium or lower: the 2026-10-08 operator directive starts at medium and raises only after a measured gain", tier, rule.Effort)
		}
	}
	for agent, rule := range block.Agents {
		if rule.Effort != "" && !slices.Contains(starting, rule.Effort) {
			t.Errorf("agents.%s.effort = %q, want medium or lower (2026-10-08 operator directive)", agent, rule.Effort)
		}
	}
}
