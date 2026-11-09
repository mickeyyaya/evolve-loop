package advisor

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func riskInput(goal string, items ...router.LaneItem) router.RouteInput {
	return router.RouteInput{
		GoalText:  goal,
		LaneItems: items,
		Cfg:       config.RoutingConfig{DeepBuildRisk: map[string][]string{"security": {"secret"}, "kernel": {"ledger"}}},
	}
}

func buildPlan(tier string, run bool) *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true, Tier: "fast"},
		{Phase: "build", Run: run, Tier: tier},
	}}
}

func TestApplyDeepBuildRisk_TierFollowsTheRiskClassOfTheLaneEvidence(t *testing.T) {
	cases := []struct {
		name, proposed, want string
		in                   router.RouteInput
	}{
		{"acceptance matches", "balanced", "deep", riskInput("", router.LaneItem{Acceptance: []string{"Redact the SECRET"}})},
		{"file matches", "", "deep", riskInput("", router.LaneItem{Files: []string{"go/x/ledger.go"}})},
		{"no match lowers deep", "deep", "balanced", riskInput("", router.LaneItem{Acceptance: []string{"fix a typo"}})},
		{"goal is the fallback evidence", "balanced", "deep", riskInput("rotate the secret")},
		{"unresolved item falls back to the goal", "balanced", "deep", riskInput("rotate the secret", router.LaneItem{Unresolved: "x"})},
		{"lane evidence wins over the goal", "deep", "balanced", riskInput("rotate the secret", router.LaneItem{Acceptance: []string{"fix a typo"}})},
		{"no evidence keeps the proposal", "deep", "deep", riskInput("")},
		{"only an unresolved item keeps the proposal", "deep", "deep", riskInput("", router.LaneItem{Unresolved: "x"})},
		{"no classes keep the proposal", "deep", "deep", router.RouteInput{GoalText: "secret"}},
	}
	for _, c := range cases {
		plan := buildPlan(c.proposed, true)
		applyDeepBuildRisk(c.in, plan)
		if got := plan.Entries[1].Tier; got != c.want {
			t.Errorf("%s: build tier %q, want %q", c.name, got, c.want)
		}
		if plan.Entries[0].Tier != "fast" {
			t.Errorf("%s: a non-build entry changed to %q", c.name, plan.Entries[0].Tier)
		}
	}
}

func TestApplyDeepBuildRisk_ASkippedBuildKeepsItsTier(t *testing.T) {
	plan := buildPlan("balanced", false)
	applyDeepBuildRisk(riskInput("rotate the secret"), plan)
	if plan.Entries[1].Tier != "balanced" {
		t.Errorf("a build that does not run changed to %q", plan.Entries[1].Tier)
	}
}

func TestReplayPlanFromResponse_AppliesTheDeepBuildRiskRule(t *testing.T) {
	raw := `[{"phase":"scout","run":true,"justification":"x"},{"phase":"build","run":true,"justification":"x","tier":"balanced"},{"phase":"audit","run":true,"justification":"x"}]`
	plan, _, err := ReplayPlanFromResponse(raw, riskInput("rotate the secret"), router.DefaultShipFloor())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range plan.Entries {
		if e.Phase == "build" && e.Tier != "deep" {
			t.Errorf("replayed build tier %q, want deep", e.Tier)
		}
	}
}

func TestRiskClassNames_AreSorted(t *testing.T) {
	if got := riskClassNames(map[string][]string{"b": nil, "a": nil}); got != "a, b" {
		t.Errorf("riskClassNames = %q, want %q", got, "a, b")
	}
}

func TestDeepTierPolicyLine_NamesClassesOrKeepsTheBlanketRule(t *testing.T) {
	if got := deepTierPolicyLine(nil); !strings.Contains(got, "build, tdd, audit") {
		t.Errorf("no classes must keep the blanket line: %q", got)
	}
	got := deepTierPolicyLine(map[string][]string{"security": {"x"}})
	if !strings.Contains(got, "security") || strings.Contains(got, "build, tdd, audit") {
		t.Errorf("classes line = %q", got)
	}
}
