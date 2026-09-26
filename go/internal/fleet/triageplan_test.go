package fleet

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func scopeIDs(spec CycleSpec) map[string]bool {
	ids := map[string]bool{}
	for _, id := range strings.Split(spec.Env[ipcenv.FleetScopeKey], ",") {
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

func TestPlanFromTriage_DisjointScopesAcrossLanes(t *testing.T) {
	decisionJSON := []byte(`{"committed_floors":["bridge","core","audit"]}`)
	specs, _, err := PlanFromTriage(decisionJSON, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("len(specs) = %d, want 2 (3 disjoint floors spread across 2 lanes, per PlanCycles' least-loaded partitioning)", len(specs))
	}
	seen := map[string]bool{}
	for i, spec := range specs {
		ids := scopeIDs(spec)
		if len(ids) == 0 {
			t.Errorf("spec[%d] has empty EVOLVE_FLEET_SCOPE — every launched lane must be scoped", i)
		}
		for id := range ids {
			if seen[id] {
				t.Errorf("todo id %q appears in more than one lane spec — lanes must be pairwise disjoint", id)
			}
			seen[id] = true
		}
	}
	for _, want := range []string{"bridge", "core", "audit"} {
		if !seen[want] {
			t.Errorf("scoped todo ids = %v, missing floor %q", seen, want)
		}
	}
}

func TestPlanFromTriage_FallsBackToCardPackagesWhenFloorsAbsent(t *testing.T) {
	decisionJSON := []byte(`{}`)
	specs, _, err := PlanFromTriage(decisionJSON, []string{"core", "audit"}, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("len(specs) = %d, want 2 (2 disjoint card-package fallbacks, one lane each)", len(specs))
	}
	seen := map[string]bool{}
	for _, spec := range specs {
		for id := range scopeIDs(spec) {
			seen[id] = true
		}
	}
	if !seen["core"] || !seen["audit"] {
		t.Errorf("scoped todo ids = %v, want both %q and %q (card-package fallback when floors are absent)", seen, "core", "audit")
	}
}

func TestPlanFromTriage_MalformedDecisionJSON_RejectsNotGuesses(t *testing.T) {
	truncated := []byte(`{"committed_floors":[`)
	specs, _, err := PlanFromTriage(truncated, []string{"core"}, 3, nil)
	if err == nil {
		t.Fatalf("PlanFromTriage(malformed) returned nil error — want an explicit parse error so the caller falls back to sequential with a WARN, never a silent guess")
	}
	if len(specs) != 0 {
		t.Errorf("PlanFromTriage(malformed) returned %d specs, want 0 (malformed input must never schedule unscoped lanes)", len(specs))
	}
}

func TestPlanFromTriage_EmptyInputsNeverOverSchedule(t *testing.T) {
	t.Run("empty-floors-and-empty-cards-yields-zero-specs", func(t *testing.T) {
		specs, _, err := PlanFromTriage([]byte(`{"committed_floors":[]}`), nil, 3, nil)
		if err != nil {
			t.Fatalf("PlanFromTriage returned error: %v", err)
		}
		if len(specs) != 0 {
			t.Errorf("len(specs) = %d, want 0 (no floors, no cards — nothing to schedule)", len(specs))
		}
	})
	t.Run("single-floor-count-three-yields-exactly-one-spec", func(t *testing.T) {
		specs, _, err := PlanFromTriage([]byte(`{"committed_floors":["bridge"]}`), nil, 3, nil)
		if err != nil {
			t.Fatalf("PlanFromTriage returned error: %v", err)
		}
		if len(specs) != 1 {
			t.Errorf("len(specs) = %d, want 1 (empty buckets yield NO spec — never pad to count)", len(specs))
		}
	})
}

func TestPlanFromTriage_ProductionFixtureTopNOnlyFallback(t *testing.T) {
	// A real triage-decision.json shape: top_n cards only, no committed_floors, and nil cardPackages.
	decisionJSON := []byte(`{
		"cycle": 464,
		"top_n": [
			{"id": "fleet-policy-block", "action": "Add FleetPolicy block."},
			{"id": "fleet-policy-docs", "action": "Document the fleet block."}
		],
		"deferred": [{"id": "cycle-366-failed-ship"}],
		"dropped": null,
		"projected_by_orchestrator": true
	}`)
	specs, _, err := PlanFromTriage(decisionJSON, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage(top_n-only production fixture) returned error: %v, want nil — a real committed-decision shape with no committed_floors must still plan, not error", err)
	}
	if len(specs) == 0 {
		t.Fatalf("PlanFromTriage(top_n-only production fixture) returned 0 specs — a real production triage-decision.json with top_n cards and no committed_floors/cardPackages must still yield at least one lane (the D1 severity amplifier)")
	}
	union := map[string]bool{}
	for _, spec := range specs {
		for id := range scopeIDs(spec) {
			union[id] = true
		}
	}
	for _, want := range []string{"fleet-policy-block", "fleet-policy-docs"} {
		if !union[want] {
			t.Errorf("scoped ids = %v, missing top_n card id %q", union, want)
		}
	}
}

func TestPlanFromTriage_SingleTopNCardCountFourYieldsOneLane(t *testing.T) {
	decisionJSON := []byte(`{"top_n":[{"id":"fleet-policy-block","action":"x"}]}`)
	specs, _, err := PlanFromTriage(decisionJSON, nil, 4, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1 (a single top_n card at count=4 must yield exactly one lane, never 4 padded lanes)", len(specs))
	}
	union := scopeIDs(specs[0])
	if !union["fleet-policy-block"] || len(union) != 1 {
		t.Errorf("scoped ids = %v, want exactly {fleet-policy-block}", union)
	}
}
