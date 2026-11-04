//go:build acs

package cycle2140

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func routingPlanTests(t *testing.T, pattern string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./cmd/evolve",
		Pattern: pattern,
		Names:   names,
	})
}

func TestC2140_001_RoutingPlanAgainstAFakeBridgePrintsThePlanAndLeavesTheCycleUntouched(t *testing.T) {
	routingPlanTests(t,
		"^TestRoutingPlan_PrintsParsedPlanAndWritesNothingUnderTheCycle$",
		"TestRoutingPlan_PrintsParsedPlanAndWritesNothingUnderTheCycle")
}

func TestC2140_002_RoutingPlanSendsThePromptTheCycleComposerBuildsForTheSameInput(t *testing.T) {
	routingPlanTests(t,
		"^(TestRoutingPlan_SendsTheCyclesOwnComposedPrompt|TestRoutingPlan_GoalTextOverridesTheRecordedGoal)$",
		"TestRoutingPlan_SendsTheCyclesOwnComposedPrompt",
		"TestRoutingPlan_GoalTextOverridesTheRecordedGoal")
}

func TestC2140_003_RoutingPlanExitsOneForAnUnknownCycleAndTwoWithTheCauseWhenTheAdvisorFails(t *testing.T) {
	routingPlanTests(t,
		"^(TestRoutingPlan_UnknownCycleExitsOneWithoutLaunchingTheAdvisor|TestRoutingPlan_AdvisorLaunchFailureExitsTwoWithTheCause|TestRoutingPlan_AdvisorWiringFailureIsNotAnUnknownCycle)$",
		"TestRoutingPlan_UnknownCycleExitsOneWithoutLaunchingTheAdvisor",
		"TestRoutingPlan_AdvisorLaunchFailureExitsTwoWithTheCause",
		"TestRoutingPlan_AdvisorWiringFailureIsNotAnUnknownCycle")
}

// acs-predicate: config-check
func TestC2140_004_RuntimeReferenceDocumentsRoutingPlanWithItsFlagsAndEveryExitCode(t *testing.T) {
	ref := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	raw, err := os.ReadFile(ref)
	if err != nil {
		t.Fatal(err)
	}
	var entry string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "evolve routing plan") {
			entry = line
			break
		}
	}
	if entry == "" {
		t.Fatalf("RED: %s does not document `evolve routing plan`", ref)
	}
	for _, flag := range []string{"--cycle N", "--goal-text"} {
		if !strings.Contains(entry, flag) {
			t.Errorf("the routing plan entry does not name %s:\n%s", flag, entry)
		}
	}
	for _, code := range []string{"0", "1", "2", "10"} {
		if !regexp.MustCompile(`(?i)\bexit ` + code + `\b`).MatchString(entry) {
			t.Errorf("the routing plan entry does not state exit %s:\n%s", code, entry)
		}
	}
	if !strings.Contains(entry, "runs/cycle-") {
		t.Errorf("the routing plan entry does not state that nothing is written under runs/cycle-N:\n%s", entry)
	}
}
