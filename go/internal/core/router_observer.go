package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const routerAgentLabel = "router"

func (o *Orchestrator) observedPlan(ctx context.Context, cs CycleState, cycle int, in router.RouteInput) (*router.PhasePlan, error) {
	defer o.observeRouter(ctx, cs, cycle)()
	return o.planner.Plan(in)
}

func (cr *cycleRun) pauseAtRouterWall(next Phase) error {
	if !cr.routerWall.HasQuota() {
		return nil
	}
	benchErr := cr.benchQuotaFamilies(cr.routerWall)
	cause := WalkError{Wall: cr.routerWall, Err: errors.New("router plan ended at a usage-limit wall")}
	return errors.Join(cr.pauseForQuota(next, PhaseResponse{Phase: string(next)}, 1, cause), benchErr)
}

func (cr *cycleRun) benchQuotaFamilies(wall Wall) error {
	if !policy.LoadOrDefaults(filepath.Join(cr.req.ProjectRoot, ".evolve", "policy.json"), io.Discard).CLIHealthConfig().On() {
		return nil
	}
	store := clihealth.NewStore(cr.req.ProjectRoot, cr.o.now)
	var errs error
	for _, end := range wall.Ends {
		if !clihealth.QuotaPattern(end.Pattern) {
			continue
		}
		if _, err := store.BenchWall(llmroute.Family(end.CLI), end.Pattern, ""); err != nil {
			errs = errors.Join(errs, fmt.Errorf("bench %s at the router quota wall: %w", end.CLI, err))
		}
	}
	return errs
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
