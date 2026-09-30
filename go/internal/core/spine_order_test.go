package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
)

func TestSpineNext_WalksCanonicalSpine(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
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
	for _, p := range []Phase{PhaseRetro, PhaseDebugger, PhaseSwarmPlan, PhaseEnd} {
		if next, ok := sm.spineNext(p); ok {
			t.Errorf("spineNext(%s) = (%s,true), want miss — %s is not a linear-spine phase", p, next, p)
		}
	}
}

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
