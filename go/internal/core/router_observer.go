package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const routerAgentLabel = "router"

func (o *Orchestrator) observedPlan(ctx context.Context, cs CycleState, cycle int, in router.RouteInput) (*router.PhasePlan, error) {
	defer o.observeRouter(ctx, cs, cycle)()
	return o.planner.Plan(in)
}

func (cr *cycleRun) observedRePlan(planner rePlanner, in router.RouteInput) (*router.PhasePlan, error) {
	defer cr.o.observeRouter(cr.ctx, cr.cs, cr.cycle)()
	return planner.RePlan(in)
}

func (o *Orchestrator) observeRouter(ctx context.Context, cs CycleState, cycle int) func() {
	stop := o.observer.Start(ctx, routerAgentLabel, PhaseRequest{Workspace: cs.WorkspacePath, Cycle: cycle, RunID: cs.RunID})
	if stop == nil {
		return func() {}
	}
	return stop
}
