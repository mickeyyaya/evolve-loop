//go:build acs

package cycle981

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func hasMultiLaneGroup(plan [][]string) bool {
	for _, g := range plan {
		if len(g) > 1 {
			return true
		}
	}
	return false
}

func TestC981_001_SalvagedComposerBehaves(t *testing.T) {
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}})

	calls := 0
	verify := func(laneIDs []string) bool {
		calls++
		return !contains(laneIDs, "L2")
	}
	landed, ejected := q.ResolveCulprit(verify)

	if !contains(landed, "L1") || !contains(landed, "L3") {
		t.Errorf("expected L1 and L3 to land, got landed=%v", landed)
	}
	if contains(landed, "L2") {
		t.Errorf("poisoned lane L2 must not land, got landed=%v", landed)
	}
	if len(ejected) != 1 || ejected[0] != "L2" {
		t.Errorf("expected exactly L2 ejected, got ejected=%v", ejected)
	}
	if calls > 6 {
		t.Errorf("verify called %d times for 3 lanes — expected linear NNFI (<=6), not a bisection sweep", calls)
	}

	w := fleet.NewPrefixQueue()
	if got := w.Window(); got != 3 {
		t.Errorf("initial window = %d, want 3", got)
	}
	w.OnGreen()
	w.OnGreen()
	if got := w.Window(); got != 5 {
		t.Errorf("window after 2 greens = %d, want 5", got)
	}
	w.OnRed()
	w.OnRed()
	if got := w.Window(); got != 1 {
		t.Errorf("window after two reds = %d, want 1", got)
	}
	w.OnRed()
	if got := w.Window(); got != 1 {
		t.Errorf("window after red at floor = %d, want 1 (floor)", got)
	}
}

func TestC981_002_ShipWiringRoutesThroughComposer(t *testing.T) {
	lanes := []fleet.LaneCandidate{
		{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}},
		{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}},
	}

	perLaneCfg := policy.Policy{}.FleetConfig()
	perLanePlan := ship.PlanLanding(perLaneCfg, lanes)
	if hasMultiLaneGroup(perLanePlan) {
		t.Errorf("per-lane plan must land each lane independently (no multi-lane group), got %v", perLanePlan)
	}
	if len(perLanePlan) != len(lanes) {
		t.Errorf("per-lane plan must have one landing per lane, got %d groups for %d lanes: %v", len(perLanePlan), len(lanes), perLanePlan)
	}

	pqCfg := policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}.FleetConfig()
	pqPlan := ship.PlanLanding(pqCfg, lanes)
	if !hasMultiLaneGroup(pqPlan) {
		t.Errorf("prefix-queue plan must compose the two lanes (a multi-lane prefix group), got %v — composer appears INERT", pqPlan)
	}
	ref := fleet.NewPrefixQueue()
	for _, l := range lanes {
		ref.Enqueue(l)
	}
	if want := ref.ComposePrefixes(); !reflect.DeepEqual(pqPlan, want) {
		t.Errorf("prefix-queue plan = %v, want fleet.PrefixQueue.ComposePrefixes() output %v", pqPlan, want)
	}

	if reflect.DeepEqual(perLanePlan, pqPlan) {
		t.Errorf("per-lane and prefix-queue plans are identical (%v) — the ship seam ignores landing mode; composer is not wired", perLanePlan)
	}

	if got := ship.PlanLanding(perLaneCfg, nil); len(got) != 0 {
		t.Errorf("per-lane plan for no lanes = %v, want empty", got)
	}
	if got := ship.PlanLanding(pqCfg, nil); len(got) != 0 {
		t.Errorf("prefix-queue plan for no lanes = %v, want empty", got)
	}
}

func TestC981_003_LandingPolicyVocabulary(t *testing.T) {
	if got := (policy.Policy{}).FleetConfig().Landing; got != "per-lane" {
		t.Errorf("default resolved landing = %q, want %q", got, "per-lane")
	}

	if got := (policy.Policy{Fleet: &policy.FleetPolicy{Landing: "per-lane"}}).FleetConfig().Landing; got != "per-lane" {
		t.Errorf("explicit per-lane resolved = %q, want %q", got, "per-lane")
	}

	if got := (policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}).FleetConfig().Landing; got != "prefix-queue" {
		t.Errorf("prefix-queue resolved = %q, want %q", got, "prefix-queue")
	}

	bogus := (policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefixqueue"}}).FleetConfig()
	if bogus.Landing != "per-lane" {
		t.Errorf("unknown landing resolved = %q, want fail-safe %q", bogus.Landing, "per-lane")
	}
	warned := false
	for _, w := range bogus.Warnings {
		if len(w) > 0 && (containsSub(w, "landing") || containsSub(w, "Landing")) {
			warned = true
			break
		}
	}
	if !warned {
		t.Errorf("unknown landing value must surface a warning; got Warnings=%v", bogus.Warnings)
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
