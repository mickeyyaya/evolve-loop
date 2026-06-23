package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// endFromScoutStrategy proposes ENDING the cycle from scout (a no-ship
// convergence — e.g. scout found nothing to do) and otherwise defers to the
// static spine.
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

// noShipPlanner returns a whole-cycle plan that runs only scout (no ship), so
// planRunsShip is false and early-exit is permitted by the kernel.
type noShipPlanner struct{}

func (noShipPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: string(PhaseScout), Run: true}}}, nil
}

// shipIntentStrategy ALSO proposes end from scout, but is paired with a planner
// that runs ship — proving the kernel REFUSES the early-exit and falls through
// to the spine (the safety invariant end-to-end through RunCycle).
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

// TestRunCycle_EarlyExit_NoShipConvergence proves end-to-end through RunCycle:
// when the advisor proposes end from scout AND the plan does not run ship, the
// cycle terminates after scout — build/audit/ship never execute.
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

// TestRunCycle_EarlyExit_RefusedWhenShipPlanned is the safety invariant
// end-to-end: even when the advisor proposes end from scout, a ship-intended
// plan makes the kernel REFUSE the early-exit — the spine runs and ship executes.
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
	// Ship-intended ⇒ early-exit refused ⇒ build + audit + ship still run.
	for _, p := range []Phase{PhaseBuild, PhaseAudit, PhaseShip} {
		if fr := runners[p].(*fakeRunner); fr.calls == 0 {
			t.Errorf("%s did not run — a ship-intended cycle must NOT early-exit (audit-before-ship is non-bypassable)", p)
		}
	}
}
