package ship

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// planlanding_test.go — default-tag coverage for the PlanLanding wiring seam
// (ADR-0069: the acs-tagged go/acs/cycle981 gate-wiring predicate does NOT run
// under `go test ./internal/...`, so repo-wide apicover flags PlanLanding as
// uncovered). This unit test pins the same routing contract under default tags.

// TestPlanLanding_RoutesOnLandingMode pins the wiring seam: per-lane yields one
// singleton group per lane (legacy, byte-identical), prefix-queue routes through
// fleet.PrefixQueue, and the two modes produce observably different plans.
func TestPlanLanding_RoutesOnLandingMode(t *testing.T) {
	lanes := []fleet.LaneCandidate{
		{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"a/a.go"}},
		{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"b/b.go"}},
	}

	perLane := PlanLanding(policy.Policy{}.FleetConfig(), lanes)
	if want := [][]string{{"L1"}, {"L2"}}; !reflect.DeepEqual(perLane, want) {
		t.Errorf("per-lane plan = %v, want %v (one singleton per lane)", perLane, want)
	}

	pqCfg := policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}.FleetConfig()
	pq := PlanLanding(pqCfg, lanes)
	ref := fleet.NewPrefixQueue()
	for _, l := range lanes {
		ref.Enqueue(l)
	}
	if want := ref.ComposePrefixes(); !reflect.DeepEqual(pq, want) {
		t.Errorf("prefix-queue plan = %v, want composer output %v", pq, want)
	}
	if reflect.DeepEqual(perLane, pq) {
		t.Errorf("per-lane and prefix-queue plans are identical (%v) — seam ignores mode", perLane)
	}
}

// TestPlanLanding_EmptyLanes pins the edge case: no lanes yields an empty plan in
// both modes, never a panic.
func TestPlanLanding_EmptyLanes(t *testing.T) {
	perLaneCfg := policy.Policy{}.FleetConfig()
	pqCfg := policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}.FleetConfig()
	if got := PlanLanding(perLaneCfg, nil); len(got) != 0 {
		t.Errorf("per-lane plan for no lanes = %v, want empty", got)
	}
	if got := PlanLanding(pqCfg, nil); len(got) != 0 {
		t.Errorf("prefix-queue plan for no lanes = %v, want empty", got)
	}
}
