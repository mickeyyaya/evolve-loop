package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	removeRouterWall(os.Stderr, wallPath)
	plan, err := o.planner.Plan(in)
	if wall := walkedWall(err); err != nil && wall.HasQuota() {
		if data, merr := json.Marshal(wall); merr == nil {
			writeRouterWall(os.Stderr, wallPath, data)
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
	removeRouterWall(os.Stderr, wallPath)
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
			benchFamily(os.Stderr, store, end)
		}
	}
}

func benchFamily(w io.Writer, store *clihealth.Store, end FamilyEnd) {
	if _, err := store.BenchWall(llmroute.Family(end.CLI), end.Pattern, ""); err != nil {
		fmt.Fprintf(w, "[router] WARN bench %s at its usage wall: %v\n", end.CLI, err)
	}
}

func removeRouterWall(w io.Writer, path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(w, "[router] WARN remove the quota wall %s: %v\n", path, err)
	}
}

func writeRouterWall(w io.Writer, path string, data []byte) {
	if err := writeArtifactAtomically(path, data); err != nil {
		fmt.Fprintf(w, "[router] WARN write the quota wall %s: %v\n", path, err)
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
