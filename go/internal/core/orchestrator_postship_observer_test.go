package core

// orchestrator_postship_observer_test.go — RED contract for cycle-574 Task 1
// (fix-memo-phase-tier-envelope, inbox weight 0.95 critical) SECOND half.
//
// Context. Cycle-573 already closed the tier half of this task: the memo pin's
// model tier now satisfies its profile envelope (internal/policy
// memo_envelope_config_test.go — GREEN). But the inbox item asks for TWO fixes;
// the tier alignment was only the first. The second, still unimplemented:
//
//	"classify PASS-side post-ship observer phases (memo, post-ship-monitor) as
//	 non-fatal: a failure there downgrades to a WARN diagnostic on an
//	 already-shipped cycle, never a cycle-level failure."
//
// Today, a memo phase that fails AFTER a healthy ship (the exact envelope-error
// shape, or any other non-infra error) flows through dispatch's abort arm
// (cyclerun_dispatch.go: wrapCycleLevelError) and turns a shipped cycle
// abnormal — the worktree is preserved, wave accounting reports 0/N ok, retro
// fires on a healthy ship. optionalInfraSkip does NOT cover this: it degrades
// only INFRA-shaped errors (artifact timeout / transient bridge), never a
// policy/logic error, and it is blind to whether ship already succeeded.
//
// The contract these tests pin — a NEW orchestrator classifier
//
//	func (o *Orchestrator) postShipObserverSkip(p Phase, shipped bool) bool
//
// which returns true (degrade the failed phase to WARN + advance instead of
// aborting) iff ALL hold:
//
//  1. shipped == true — ship has already recorded a PASS this cycle. A failure
//     BEFORE ship is never swallowed (nothing shipped to protect yet).
//  2. p is a best-effort post-ship observer — a RoleControl observer phase
//     (memo / post-ship-monitor), NOT ship/build/audit/etc.
//  3. p is NOT configured-mandatory and NOT in the resolved ship floor — the
//     skip can never weaken the integrity floor (same guard optionalInfraSkip
//     uses).
//
// RED today: postShipObserverSkip is undefined, so package core's test build
// fails to compile — the intended RED signal before Builder implements the
// classifier and wires it into the dispatch abort path.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// postShipObserverOrchestrator builds a minimal orchestrator whose catalog
// knows a post-ship OPTIONAL memo observer and whose config marks the spine
// (build/audit/ship) mandatory — the shape the classifier reasons over. No
// storage/ledger/runners are needed: the classifier is a pure decision over
// (catalog, cfg, phase, shipped).
func postShipObserverOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "memo", Optional: true, After: "ship"},
	})
	if err != nil {
		t.Fatalf("setup: catalog merge: %v", err)
	}
	cfg := config.RoutingConfig{Mandatory: []string{"build", "audit", "ship"}}
	return NewOrchestrator(nil, nil, nil, WithCatalog(cat), WithRouting(cfg, nil))
}

// TestPostShipObserverSkip_MemoAfterShipIsNonFatal — AC-2a (the core fix): a
// memo failure on an ALREADY-SHIPPED cycle is degraded (skip==true) so the
// cycle keeps its shipped/PASS outcome. Exercises the real classifier over the
// real catalog/cfg — not a source grep.
func TestPostShipObserverSkip_MemoAfterShipIsNonFatal(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if !o.postShipObserverSkip(Phase("memo"), true) {
		t.Error("a memo failure AFTER a healthy ship must be non-fatal (degrade to WARN + advance), " +
			"never a cycle-level failure that turns a shipped cycle abnormal (inbox memo-phase-tier-envelope)")
	}
}

// TestPostShipObserverSkip_MemoBeforeShipStaysFatal — AC-2b (negative /
// anti-no-op): the skip is scoped to POST-ship. A memo failure when ship has
// NOT yet succeeded (shipped==false) must NOT be swallowed — otherwise a broken
// memo could mask a genuine pre-ship failure. This is the discriminator that
// forbids a degenerate "memo is always skipped" implementation.
func TestPostShipObserverSkip_MemoBeforeShipStaysFatal(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if o.postShipObserverSkip(Phase("memo"), false) {
		t.Error("a memo failure BEFORE ship must stay cycle-fatal — the non-fatal downgrade applies only once a " +
			"ship has landed (shipped==true); swallowing it pre-ship would hide real failures")
	}
}

// TestPostShipObserverSkip_MandatoryFloorNeverSkipped — AC-2c (floor guard): a
// configured-mandatory / ship-floor phase (build) is NEVER post-ship-skipped,
// even with shipped==true. The downgrade must never weaken ship ⇒ build ∧ audit
// ∧ tdd — the same invariant optionalInfraSkip protects.
func TestPostShipObserverSkip_MandatoryFloorNeverSkipped(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if o.postShipObserverSkip(Phase("build"), true) {
		t.Error("a mandatory/floor phase (build) must never be post-ship-skipped — the observer downgrade must not " +
			"weaken the integrity floor")
	}
}

// TestPostShipObserverSkip_ShipItselfNeverSkipped — AC-2d (anchor guard): ship
// itself is never post-ship-skipped (it is the ship gate; a ship failure is
// resolved by the ShipError recovery seam, not swallowed).
func TestPostShipObserverSkip_ShipItselfNeverSkipped(t *testing.T) {
	t.Parallel()
	o := postShipObserverOrchestrator(t)
	if o.postShipObserverSkip(PhaseShip, true) {
		t.Error("ship must never be post-ship-skipped — it is the ship gate itself, not a best-effort observer")
	}
}
