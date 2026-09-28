package ship

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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
