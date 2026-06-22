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
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestOrchestrator_ThreadsIntentRequiredToPlannerInput(t *testing.T) {
	runIntent := func(t *testing.T, wfCfg policy.WorkflowConfig) router.RouteInput {
		t.Helper()
		st := &fakeStorage{state: State{LastCycleNumber: 0}}
		led := &fakeLedger{}
		runners := buildRunners(nil)
		cfg := shadowCfg(config.StageAdvisory)
		cfg.Mode = config.ModeDynamicLLM
		cp := &capturingPlanner{}
		o := NewOrchestrator(st, led, runners, WithRouting(cfg, router.StaticPreset{}), WithPlanner(cp), WithWorkflowConfig(wfCfg))

		if _, err := o.RunCycle(context.Background(), CycleRequest{
			ProjectRoot:           t.TempDir(),
			GoalHash:              "g",
			DisableWorkspaceGuard: true,
		}); err != nil {
			t.Fatalf("RunCycle: %v", err)
		}
		if cp.calls == 0 {
			t.Fatal("planner was never consulted")
		}
		return cp.got
	}

	// Positive: PhaseEnables["intent"]="on" must reach the plan-clamp input.
	got := runIntent(t, policy.WorkflowConfig{PhaseEnables: map[string]string{"intent": "on"}})
	if !got.IntentRequired {
		t.Errorf("planIn.IntentRequired=false with PhaseEnables[intent]=on; floor clamp cannot force intent without it")
	}

	// Negative: without the enable the bit stays false.
	got = runIntent(t, policy.WorkflowConfig{})
	if got.IntentRequired {
		t.Errorf("planIn.IntentRequired=true without intent enable; must default false")
	}
}
