package core

// spine_order_test.go — PA-BIG S5 (ADR-0058): the linear transition spine is a
// DATA table (spineOrder), walked by spineNext, instead of a per-phase switch in
// Next. spineOrder is a config-INDEPENDENT trust anchor (like the legality
// graph): config SELECTS among legal edges (on_pass/on_fail), it cannot move the
// spine. The full Next() byte-identity is proven by the transition oracle
// (TestTransitionKernelOracle_Next); this pins the SSOT table directly.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/kerneltest"
)

// TestSpineNext_WalksCanonicalSpine asserts spineNext returns each phase's
// canonical linear successor — the exact values the pre-S5 Next switch hardcoded
// — and misses for phases that are not on the linear spine (sentinels, swarm).
func TestSpineNext_WalksCanonicalSpine(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine() // no injected spine → the canonical literal fallback
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
		got, ok := sm.spineNext(p)
		if !ok || got != exp {
			t.Errorf("spineNext(%s) = (%s,%v), want (%s,true)", p, got, ok, exp)
		}
	}
	// Phases that are NOT linear must miss — Next handles them explicitly
	// (sentinels → end) or as a no-successor error (swarm-plan).
	for _, p := range []Phase{PhaseRetro, PhaseDebugger, PhaseSwarmPlan, PhaseEnd} {
		if next, ok := sm.spineNext(p); ok {
			t.Errorf("spineNext(%s) = (%s,true), want miss — %s is not a linear-spine phase", p, next, p)
		}
	}
}

// TestSpineNext_InjectedConfigSpine proves the spine is config-driven (DDK-3):
// the spine LOADED from the registry (config.spine_order) drives spineNext for
// every consecutive pair — no hardcoded phase names, so a registry rename is
// followed automatically.
func TestSpineNext_InjectedConfigSpine(t *testing.T) {
	t.Parallel()
	spine := spinePhasesFrom(kerneltest.Load(t).Spine())
	if len(spine) < 2 {
		t.Fatal("reference registry must declare a multi-phase spine")
	}
	sm := NewStateMachine().WithSpine(spine)
	for i := 0; i+1 < len(spine); i++ {
		if got, ok := sm.spineNext(spine[i]); !ok || got != spine[i+1] {
			t.Errorf("injected spine: spineNext(%s) = (%s,%v), want (%s,true)", spine[i], got, ok, spine[i+1])
		}
	}
}
