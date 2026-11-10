package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const (
	routerAgentLabel    = "router"
	routerQuotaWallFile = "router-quota-wall.json"
)

func (o *Orchestrator) observedPlan(ctx context.Context, cs CycleState, cycle int, in router.RouteInput) (*router.PhasePlan, error) {
	defer o.observeRouter(ctx, cs, cycle)()
	wallPath := filepath.Join(cs.WorkspacePath, routerQuotaWallFile)
	os.Remove(wallPath)
	plan, err := o.planner.Plan(in)
	if wall := walkedWall(err); err != nil && wall.HasQuota() {
		if data, merr := json.Marshal(wall); merr == nil {
			writeArtifactAtomically(wallPath, data)
		}
	}
	return plan, err
}

func (cr *cycleRun) pauseAtRouterWall(next Phase) error {
	wallPath := filepath.Join(cr.cs.WorkspacePath, routerQuotaWallFile)
	data, err := os.ReadFile(wallPath)
	if err != nil {
		return nil
	}
	os.Remove(wallPath)
	var wall Wall
	if json.Unmarshal(data, &wall) != nil || !wall.HasQuota() {
		return nil
	}
	cr.benchQuotaFamilies(wall)
	cause := WalkError{Wall: wall, Err: errors.New("router plan ended at a usage-limit wall")}
	return cr.pauseForQuota(next, PhaseResponse{Phase: string(next)}, 1, cause)
}

func (cr *cycleRun) benchQuotaFamilies(wall Wall) {
	if !policy.LoadOrDefaults(filepath.Join(cr.req.ProjectRoot, ".evolve", "policy.json"), io.Discard).CLIHealthConfig().On() {
		return
	}
	store := clihealth.NewStore(cr.req.ProjectRoot, cr.o.now)
	for _, end := range wall.Ends {
		if clihealth.QuotaPattern(end.Pattern) {
			store.BenchWall(llmroute.Family(end.CLI), end.Pattern, "")
		}
	}
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
