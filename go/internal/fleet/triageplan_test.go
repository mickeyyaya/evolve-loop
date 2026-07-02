package fleet

// triageplan_test.go — RED-first contract for PlanFromTriage (FLEET-AS-POLICY
// S2, salvaged from cycle 465's preserved worktree per cycle-466's operator
// T1: fix D1 empty-plan livelock + nil cardPackages). See scout-report.md
// Task 1 and .evolve/evals/s2-wave-salvage-fix-d1.md for the acceptance
// criteria this file materializes. PlanFromTriage does not exist yet in this
// worktree; every test below fails to COMPILE until Builder adds it — that
// compile failure IS the RED evidence (mirrors cycle-465's precedent, and
// cycle-464's C464_001-004 before it).

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// scopeIDs splits a launched spec's EVOLVE_FLEET_SCOPE into the set of todo
// IDs it carries, so tests can assert cross-spec disjointness without caring
// about PlanFromTriage's internal file-scope string choice.
func scopeIDs(spec CycleSpec) map[string]bool {
	ids := map[string]bool{}
	for _, id := range strings.Split(spec.Env[ipcenv.FleetScopeKey], ",") {
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

// TestPlanFromTriage_DisjointScopesAcrossLanes (AC1, positive): 3 committed
// floors partitioned over count=2 lanes must yield exactly 2 specs (PlanCycles
// spreads 3 disjoint todos across 2 buckets — never 3, never 0), every spec
// scoped (non-empty EVOLVE_FLEET_SCOPE), and no todo id repeated across
// specs. Kills a stub that returns `count` empty/identical specs.
func TestPlanFromTriage_DisjointScopesAcrossLanes(t *testing.T) {
	decisionJSON := []byte(`{"committed_floors":["bridge","core","audit"]}`)
	specs, err := PlanFromTriage(decisionJSON, nil, 2)
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

// TestPlanFromTriage_FallsBackToCardPackagesWhenFloorsAbsent (AC1, positive):
// an absent committed_floors field must fall back to the caller-supplied
// committed-card target packages, not zero specs.
func TestPlanFromTriage_FallsBackToCardPackagesWhenFloorsAbsent(t *testing.T) {
	decisionJSON := []byte(`{}`)
	specs, err := PlanFromTriage(decisionJSON, []string{"core", "audit"}, 2)
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

// TestPlanFromTriage_MalformedDecisionJSON_RejectsNotGuesses (AC3, negative):
// truncated/invalid JSON must return a non-nil error and zero specs — never a
// silently-guessed unscoped launch. Gaming fake it kills: an adapter that
// swallows the parse error and schedules `count` unscoped identical lanes.
func TestPlanFromTriage_MalformedDecisionJSON_RejectsNotGuesses(t *testing.T) {
	truncated := []byte(`{"committed_floors":[`)
	specs, err := PlanFromTriage(truncated, []string{"core"}, 3)
	if err == nil {
		t.Fatalf("PlanFromTriage(malformed) returned nil error — want an explicit parse error so the caller falls back to sequential with a WARN, never a silent guess")
	}
	if len(specs) != 0 {
		t.Errorf("PlanFromTriage(malformed) returned %d specs, want 0 (malformed input must never schedule unscoped lanes)", len(specs))
	}
}

// TestPlanFromTriage_EmptyInputsNeverOverSchedule (AC3, edge/OOD): empty
// floors AND empty cards yield zero specs (never a panic); a single floor with
// count=3 yields exactly ONE spec, not three — empty buckets yield NO spec,
// matching PlanCycles' existing contract (partition.go:17-34).
func TestPlanFromTriage_EmptyInputsNeverOverSchedule(t *testing.T) {
	t.Run("empty-floors-and-empty-cards-yields-zero-specs", func(t *testing.T) {
		specs, err := PlanFromTriage([]byte(`{"committed_floors":[]}`), nil, 3)
		if err != nil {
			t.Fatalf("PlanFromTriage returned error: %v", err)
		}
		if len(specs) != 0 {
			t.Errorf("len(specs) = %d, want 0 (no floors, no cards — nothing to schedule)", len(specs))
		}
	})
	t.Run("single-floor-count-three-yields-exactly-one-spec", func(t *testing.T) {
		specs, err := PlanFromTriage([]byte(`{"committed_floors":["bridge"]}`), nil, 3)
		if err != nil {
			t.Fatalf("PlanFromTriage returned error: %v", err)
		}
		if len(specs) != 1 {
			t.Errorf("len(specs) = %d, want 1 (empty buckets yield NO spec — never pad to count)", len(specs))
		}
	})
}

// TestPlanFromTriage_ProductionFixtureTopNOnlyFallback (AC2): a
// triage-decision.json shaped like the REAL cycle-464 artifact — top_n[].id
// cards, NO committed_floors field — with the caller-supplied cardPackages
// left nil (production's productionWavePlanFn never threads a package list)
// must still yield >=1 non-empty lane. This is the scout report's severity
// amplifier: real triage decisions commonly carry no committed_floors at
// all, so the floorless+cardless livelock is the COMMON path, not an edge
// case — D1 fires on the first wave of any real batch without this
// fallback. Gaming fake this kills: a fixture doctored WITH committed_floors
// (dodges the real-world shape that triggered D1).
func TestPlanFromTriage_ProductionFixtureTopNOnlyFallback(t *testing.T) {
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
	specs, err := PlanFromTriage(decisionJSON, nil, 2)
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

// TestPlanFromTriage_SingleTopNCardCountFourYieldsOneLane (AC6, edge/OOD): a
// triage-decision.json with exactly one top_n card and fc.Count=4 must
// produce EXACTLY 1 lane spec — PlanCycles' empty-bucket contract must hold
// through the top_n fallback path too (never pad unused lanes to fc.Count).
func TestPlanFromTriage_SingleTopNCardCountFourYieldsOneLane(t *testing.T) {
	decisionJSON := []byte(`{"top_n":[{"id":"fleet-policy-block","action":"x"}]}`)
	specs, err := PlanFromTriage(decisionJSON, nil, 4)
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
