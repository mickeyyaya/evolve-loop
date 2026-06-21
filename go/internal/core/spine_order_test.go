package core

// spine_order_test.go — PA-BIG S5 (ADR-0058): the linear transition spine is a
// DATA table (spineOrder), walked by spineNext, instead of a per-phase switch in
// Next. spineOrder is a config-INDEPENDENT trust anchor (like the legality
// graph): config SELECTS among legal edges (on_pass/on_fail), it cannot move the
// spine. The full Next() byte-identity is proven by the transition oracle
// (TestTransitionKernelOracle_Next); this pins the SSOT table directly.

import "testing"

// TestSpineNext_WalksCanonicalSpine asserts spineNext returns each phase's
// canonical linear successor — the exact values the pre-S5 Next switch hardcoded
// — and misses for phases that are not on the linear spine (sentinels, swarm).
func TestSpineNext_WalksCanonicalSpine(t *testing.T) {
	t.Parallel()
	// The linear successors the literal Next switch encoded before S5.
	want := map[Phase]Phase{
		PhaseIntent:       PhaseScout,
		PhaseScout:        PhaseTriage,
		PhaseTriage:       PhaseTDD,
		PhaseTDD:          PhaseBuildPlanner,
		PhaseBuildPlanner: PhaseBuild,
		PhaseBuild:        PhaseAudit,
		PhaseShip:         PhaseEnd,
	}
	for p, exp := range want {
		got, ok := spineNext(p)
		if !ok || got != exp {
			t.Errorf("spineNext(%s) = (%s,%v), want (%s,true)", p, got, ok, exp)
		}
	}
	// Phases that are NOT linear must miss — Next handles them explicitly
	// (sentinels → end) or as a no-successor error (swarm-plan).
	for _, p := range []Phase{PhaseRetro, PhaseDebugger, PhaseSwarmPlan, PhaseEnd} {
		if next, ok := spineNext(p); ok {
			t.Errorf("spineNext(%s) = (%s,true), want miss — %s is not a linear-spine phase", p, next, p)
		}
	}
}
