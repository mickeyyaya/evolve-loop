package core

// Cycle-238 advisory-soak defect D4, orchestrator call-site: the upfront-plan
// RouteInput (planIn) the orchestrator hands the advisor — and then passes to
// ClampPlanToFloorWith — must carry the cycle's IntentRequired bit. Without
// it the floor clamp cannot force intent into the plan, and the advisory
// override drops the operator's EVOLVE_REQUIRE_INTENT=1 gate (see
// floor_intent_test.go in internal/router for the clamp-side contract).

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestOrchestrator_ThreadsIntentRequiredToPlannerInput(t *testing.T) {
	runIntent := func(t *testing.T, env map[string]string) router.RouteInput {
		t.Helper()
		st := &fakeStorage{state: State{LastCycleNumber: 0}}
		led := &fakeLedger{}
		runners := buildRunners(nil)
		cfg := shadowCfg(config.StageAdvisory)
		cfg.Mode = config.ModeDynamicLLM
		cp := &capturingPlanner{}
		o := NewOrchestrator(st, led, runners, WithRouting(cfg, router.StaticPreset{}), WithPlanner(cp))

		if _, err := o.RunCycle(context.Background(), CycleRequest{
			ProjectRoot: t.TempDir(),
			GoalHash:    "g",
			Env:         env,
		}); err != nil {
			t.Fatalf("RunCycle: %v", err)
		}
		if cp.calls == 0 {
			t.Fatal("planner was never consulted")
		}
		return cp.got
	}

	// Positive: EVOLVE_REQUIRE_INTENT=1 must reach the plan-clamp input.
	got := runIntent(t, map[string]string{
		"EVOLVE_DISABLE_WORKSPACE_GUARD": "1",
		"EVOLVE_REQUIRE_INTENT":          "1",
	})
	if !got.IntentRequired {
		t.Errorf("planIn.IntentRequired=false with EVOLVE_REQUIRE_INTENT=1; the floor clamp cannot force intent without it")
	}

	// Negative: without the flag the bit stays false (guards against
	// unconditionally-true plumbing that would force intent on every cycle).
	got = runIntent(t, map[string]string{"EVOLVE_DISABLE_WORKSPACE_GUARD": "1"})
	if got.IntentRequired {
		t.Errorf("planIn.IntentRequired=true without EVOLVE_REQUIRE_INTENT; must default false")
	}
}
