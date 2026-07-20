//go:build acs

// Package cycle981 materializes the cycle-981 acceptance criteria for the sole
// inbox item this fleet lane is pinned to: prefix-speculation-landing-queue
// (.evolve/inbox/2026-07-13T14-21-00Z-prefix-speculation-landing-queue.json,
// weight 0.93, campaign merge-efficiency-2026-07). Per R9.3 no predicate here
// binds to any other lane's items — fleet_scope pins this lane to exactly this id.
//
// Scout split the item into two dependent, independently-verifiable tasks:
//
//	Task 1  salvage-prefix-queue-composer-core   — promote cycle-975's audited
//	        go/internal/fleet/prefixqueue.go (PASS 0.90, untracked in
//	        .evolve/worktrees/cycle-21f9f7ae-975) into the main lineage verbatim.
//	Task 2  prefix-queue-ship-wiring-and-policy   — the deferred AC4/AC5 wiring:
//	        add FleetPolicy.Landing (per-lane|prefix-queue, shadow-first closed
//	        vocab mirroring FleetPolicy.Scheduling) + the ship-phase seam that
//	        ROUTES landing through fleet.PrefixQueue when prefix-queue is selected.
//
// SUT SURFACE the Builder must add WITHOUT modifying this file (the RED contract —
// these symbols/fields do not exist yet in the main lineage, so this predicate
// package FAILS TO COMPILE now, which is the correct greenfield RED per
// go/acs/README.md "a predicate package that fails to compile is a HARD suite
// error"):
//
//	Task 1 (promote verbatim from the cycle-975 worktree, package go/internal/fleet):
//	  type PrefixQueue, LaneCandidate, RiskTier + NewPrefixQueue/Enqueue/Window/
//	  OnGreen/OnRed/ComposePrefixes/ResolveCulprit + LandingMode/ParseLandingMode.
//
//	Task 2 (new):
//	  // go/internal/policy — mirror the Scheduling closed-vocab resolver:
//	  FleetPolicy.Landing string   // json:"landing,omitempty"
//	  FleetConfig.Landing string   // resolved: "per-lane" (default) | "prefix-queue"
//	                               // unknown => "per-lane" + a surfaced Warnings entry
//
//	  // go/internal/phases/ship — the WIRING SEAM (the single function the
//	  // main-push path consults; it must NOT be a parallel getter — the postship
//	  // landing decision routes through it):
//	  func PlanLanding(cfg policy.FleetConfig, lanes []fleet.LaneCandidate) [][]string
//	  //   per-lane    => each lane lands independently (legacy, byte-identical):
//	  //                  one singleton group per lane, never a multi-lane group.
//	  //   prefix-queue=> routes the lanes through fleet.PrefixQueue.ComposePrefixes
//	  //                  so the single-writer composer owns the main-push decision.
//
// PREDICATE STYLE (cycle-85 anti-gaming rule): every predicate CALLS the SUT and
// asserts on its return value — no source-grep predicate exists here. go/internal
// is importable from go/acs (cycle-962 imports internal/core precedent).
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C981_001 (good lanes land around a poisoned middle; AIMD grows on
//	           green), C981_002 (prefix-queue mode composes two lanes into one
//	           prefix group), C981_003 (canonical modes resolve).
//	NEGATIVE → C981_001 (the poisoned lane must NOT land, and NNFI resolves in a
//	           LINEAR verify budget — no bisection sweep), C981_002 (per-lane and
//	           prefix-queue plans MUST DIFFER — a wiring that ignores config and
//	           always returns per-lane is INERT and fails this; the exact cycle-975
//	           "composer stays inert" risk the Auditor flagged), C981_003 (a bogus
//	           landing value must fail SAFE to per-lane WITH a warning — never
//	           silently enter composer mode).
//	EDGE     → C981_001 (window floors at 1 under repeated reds), C981_002
//	           (empty lane set => empty plan, both modes).
//	SEMANTIC → salvaged-composer-behavior / ship-wiring-routes-through-composer /
//	           policy-vocabulary-resolution are three DISTINCT behaviors.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 salvaged fleet.PrefixQueue present & behaves (ejection + AIMD)  → C981_001 (predicate)
//	AC2 cycle-975 predicates pass in promoted loc + fleet race green    → manual+checklist (Auditor)
//	AC3 full repo build green (go build ./...)                          → manual+checklist (Auditor)
//	AC4 ship PlanLanding routes through the composer iff prefix-queue,
//	    legacy per-lane otherwise (default byte-identical; gate-wiring)  → C981_002 (predicate)
//	AC5 policy resolves fleet.landing as closed vocab mirroring
//	    scheduling: default per-lane, prefix-queue ok, unknown fail-safe → C981_003 (predicate)
package cycle981

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// contains reports whether ids includes id.
func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// hasMultiLaneGroup reports whether any group in the landing plan holds >1 lane.
func hasMultiLaneGroup(plan [][]string) bool {
	for _, g := range plan {
		if len(g) > 1 {
			return true
		}
	}
	return false
}

// C981_001 — AC1: the salvaged composer is present in the main lineage and
// behaves per its contract. Task 1 is a verbatim promotion, so this re-exercises
// the two load-bearing behaviors (positional NNFI culprit ejection + AIMD window)
// to prove the package actually landed and works outside the cycle-975 worktree.
func TestC981_001_SalvagedComposerBehaves(t *testing.T) {
	// Positional culprit ejection: L2 is poisoned; L1/L3 land, L2 is ejected.
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}})

	calls := 0
	verify := func(laneIDs []string) bool {
		calls++
		return !contains(laneIDs, "L2") // a prefix fails iff it holds the poisoned lane
	}
	landed, ejected := q.ResolveCulprit(verify)

	// POSITIVE: both good lanes land.
	if !contains(landed, "L1") || !contains(landed, "L3") {
		t.Errorf("expected L1 and L3 to land, got landed=%v", landed)
	}
	// NEGATIVE (anti-no-op): the poisoned lane must NOT land, and it is the sole ejection.
	if contains(landed, "L2") {
		t.Errorf("poisoned lane L2 must not land, got landed=%v", landed)
	}
	if len(ejected) != 1 || ejected[0] != "L2" {
		t.Errorf("expected exactly L2 ejected, got ejected=%v", ejected)
	}
	// NEGATIVE (anti-bisection): NNFI resolves in a LINEAR verify budget.
	if calls > 6 {
		t.Errorf("verify called %d times for 3 lanes — expected linear NNFI (<=6), not a bisection sweep", calls)
	}

	// AIMD window: start 3, +1/green, halve/red, floor 1.
	w := fleet.NewPrefixQueue()
	if got := w.Window(); got != 3 {
		t.Errorf("initial window = %d, want 3", got)
	}
	w.OnGreen()
	w.OnGreen() // 3 -> 5
	if got := w.Window(); got != 5 {
		t.Errorf("window after 2 greens = %d, want 5", got)
	}
	w.OnRed() // 5 -> 2
	w.OnRed() // 2 -> 1
	if got := w.Window(); got != 1 {
		t.Errorf("window after two reds = %d, want 1", got)
	}
	// EDGE: floor holds — a further red never drives the window below 1.
	w.OnRed()
	if got := w.Window(); got != 1 {
		t.Errorf("window after red at floor = %d, want 1 (floor)", got)
	}
}

// C981_002 — AC4: the ship-phase landing seam ROUTES through the composer iff
// policy selects prefix-queue, and falls back to the legacy independent per-lane
// plan otherwise. This is the gate-WIRING proof: the two modes must produce
// OBSERVABLY DIFFERENT landing plans, so an inert wiring (config ignored, always
// per-lane — the exact cycle-975 failure the Auditor flagged) cannot pass.
func TestC981_002_ShipWiringRoutesThroughComposer(t *testing.T) {
	// Two composable (non-iffy, disjoint-file) lanes: the composer groups them.
	lanes := []fleet.LaneCandidate{
		{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}},
		{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}},
	}

	// Default policy (no fleet block) resolves to per-lane: each lane lands
	// independently — the legacy, byte-identical shape (NO multi-lane group).
	perLaneCfg := policy.Policy{}.FleetConfig()
	perLanePlan := ship.PlanLanding(perLaneCfg, lanes)
	if hasMultiLaneGroup(perLanePlan) {
		t.Errorf("per-lane plan must land each lane independently (no multi-lane group), got %v", perLanePlan)
	}
	if len(perLanePlan) != len(lanes) {
		t.Errorf("per-lane plan must have one landing per lane, got %d groups for %d lanes: %v", len(perLanePlan), len(lanes), perLanePlan)
	}

	// prefix-queue policy routes the lanes through fleet.PrefixQueue: the two
	// composable lanes appear in a single composed prefix group (composer engaged).
	pqCfg := policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}.FleetConfig()
	pqPlan := ship.PlanLanding(pqCfg, lanes)
	if !hasMultiLaneGroup(pqPlan) {
		t.Errorf("prefix-queue plan must compose the two lanes (a multi-lane prefix group), got %v — composer appears INERT", pqPlan)
	}
	// The plan must be exactly what the composer itself produces — proving the
	// seam ROUTES through fleet.PrefixQueue rather than reimplementing landing.
	ref := fleet.NewPrefixQueue()
	for _, l := range lanes {
		ref.Enqueue(l)
	}
	if want := ref.ComposePrefixes(); !reflect.DeepEqual(pqPlan, want) {
		t.Errorf("prefix-queue plan = %v, want fleet.PrefixQueue.ComposePrefixes() output %v", pqPlan, want)
	}

	// NEGATIVE (anti-inert): the two modes MUST differ for the same lanes. A
	// wiring that ignores cfg.Landing and always returns per-lane fails here.
	if reflect.DeepEqual(perLanePlan, pqPlan) {
		t.Errorf("per-lane and prefix-queue plans are identical (%v) — the ship seam ignores landing mode; composer is not wired", perLanePlan)
	}

	// EDGE: an empty lane set yields an empty plan in both modes (no panic).
	if got := ship.PlanLanding(perLaneCfg, nil); len(got) != 0 {
		t.Errorf("per-lane plan for no lanes = %v, want empty", got)
	}
	if got := ship.PlanLanding(pqCfg, nil); len(got) != 0 {
		t.Errorf("prefix-queue plan for no lanes = %v, want empty", got)
	}
}

// C981_003 — AC5: policy resolves fleet.landing as a closed vocabulary mirroring
// fleet.scheduling — default per-lane, prefix-queue accepted, an unknown value
// fails SAFE to per-lane WITH a surfaced warning (never silently enter composer
// mode). Exercises the real policy.Policy.FleetConfig() resolver.
func TestC981_003_LandingPolicyVocabulary(t *testing.T) {
	// Default (no fleet block) => per-lane, no landing warning.
	if got := (policy.Policy{}).FleetConfig().Landing; got != "per-lane" {
		t.Errorf("default resolved landing = %q, want %q", got, "per-lane")
	}

	// Explicit per-lane => per-lane.
	if got := (policy.Policy{Fleet: &policy.FleetPolicy{Landing: "per-lane"}}).FleetConfig().Landing; got != "per-lane" {
		t.Errorf("explicit per-lane resolved = %q, want %q", got, "per-lane")
	}

	// prefix-queue => prefix-queue (the opt-in is honored).
	if got := (policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}).FleetConfig().Landing; got != "prefix-queue" {
		t.Errorf("prefix-queue resolved = %q, want %q", got, "prefix-queue")
	}

	// NEGATIVE (anti-typo): a bogus value must fail safe to per-lane AND surface a
	// warning naming the rejected value — never silently opt into the composer.
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

// containsSub reports whether s contains sub (avoids importing strings for one use
// alongside reflect; keeps the assertion self-contained).
func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
