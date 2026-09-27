package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type endFromScoutStrategy struct{}

func (endFromScoutStrategy) Decide(in router.RouteInput) router.RouterDecision {
	if in.Current == string(PhaseScout) {
		return router.RouterDecision{NextPhase: string(PhaseEnd)}
	}
	return router.RouterDecision{}
}
func (endFromScoutStrategy) Recover(router.RouteInput) router.RouterDecision {
	return router.RouterDecision{}
}

// noShipPlanner runs only scout, so planRunsShip is false and early-exit is
// permitted.
type noShipPlanner struct{}

func (noShipPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: string(PhaseScout), Run: true}}}, nil
}

// shipPlanner runs the full ship-intended spine, so early-exit must be refused.
type shipPlanner struct{}

func (shipPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: string(PhaseScout), Run: true},
		{Phase: string(PhaseBuild), Run: true},
		{Phase: string(PhaseAudit), Run: true},
		{Phase: string(PhaseShip), Run: true},
	}}, nil
}

func dynamicCfg() config.RoutingConfig {
	cfg := shadowCfg(config.StageEnforce)
	cfg.Mode = config.ModeDynamicLLM // so the planner is consulted (planRunsShip reads its plan)
	return cfg
}

func TestRunCycle_EarlyExit_NoShipConvergence(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)

	o := NewOrchestrator(st, led, runners,
		WithRouting(dynamicCfg(), endFromScoutStrategy{}),
		WithPlanner(noShipPlanner{}))
	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot:           t.TempDir(),
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if indexOfPhase(res.PhasesRun, "scout") < 0 {
		t.Errorf("scout should have run before the early-exit; PhasesRun=%v", res.PhasesRun)
	}
	for _, p := range []Phase{PhaseBuild, PhaseAudit, PhaseShip} {
		if fr := runners[p].(*fakeRunner); fr.calls != 0 {
			t.Errorf("%s ran %d times — a no-ship early-exit must not reach build/audit/ship (PhasesRun=%v)", p, fr.calls, res.PhasesRun)
		}
	}
}

func TestRunCycle_EarlyExit_RefusedWhenShipPlanned(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)

	o := NewOrchestrator(st, led, runners,
		WithRouting(dynamicCfg(), endFromScoutStrategy{}),
		WithPlanner(shipPlanner{}))
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot:           t.TempDir(),
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	for _, p := range []Phase{PhaseBuild, PhaseAudit, PhaseShip} {
		if fr := runners[p].(*fakeRunner); fr.calls == 0 {
			t.Errorf("%s did not run — a ship-intended cycle must NOT early-exit (audit-before-ship is non-bypassable)", p)
		}
	}
}
