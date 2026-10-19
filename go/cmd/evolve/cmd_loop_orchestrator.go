package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const laneTimeWindow = 50

func newLaneLeases(projectRoot string, oc policy.OrchestratorConfig, publish orchestrator.Publish, warn io.Writer) (*orchestrator.Leases, orchestrator.LeaseConfig) {
	runs := filepath.Dir(paths.RunWorkspace(projectRoot, 0))
	times, err := orchestrator.LaneTimes(runs, laneTimeWindow)
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: orchestrator: no lane times (%v); the lease deadline is the %d-minute floor\n", err, oc.DeadlineFloorMinutes)
	}
	cfg := orchestrator.LeaseConfig{
		Stage:    oc.Stage,
		Deadline: orchestrator.DeadlineOf(times, time.Duration(oc.DeadlineFloorMinutes)*time.Minute),
		Stall:    time.Duration(oc.StallMinutes) * time.Minute,
	}
	fmt.Fprintf(warn, "[loop] orchestrator: lane lease (stage %s): deadline %s from %d lane time(s), stall %s\n", cfg.Stage, cfg.Deadline, len(times), cfg.Stall)
	return orchestrator.NewLeases(cfg, orchestrator.SystemClock{}, orchestrator.RunsHeartbeat(runs), publish), cfg
}

func (b *loopBatchCoordinator) laneLeases(oc policy.OrchestratorConfig) *orchestrator.Leases {
	if b.leases == nil {
		b.leases = map[string]*orchestrator.Leases{}
	}
	if l, ok := b.leases[oc.Stage]; ok {
		return l
	}
	l, _ := newLaneLeases(b.cfg.ProjectRoot, oc, b.deps.Signals.Emit, b.stderr)
	b.leases[oc.Stage] = l
	return l
}

func (b *loopBatchCoordinator) shadowLease(oc policy.OrchestratorConfig, launch fleet.LaunchFn) fleet.LaunchFn {
	oc.Stage = orchestrator.StageShadow
	return b.laneLeases(oc).Watch(launch, nil)
}

func (b *loopBatchCoordinator) poolHalted() bool {
	return b.ctx.Err() != nil || loopchain.BrakeEngaged(b.cfg.EvolveDir)
}

func (b *loopBatchCoordinator) orchestratorPool(fc policy.FleetConfig, launch fleet.LaunchFn, iteration int) orchestrator.Pool {
	return orchestrator.Pool{
		Target: fc.Count, Concurrency: fc.Concurrency, MaxLanes: fc.Orchestrator.RunMaxLanes,
		MinLanes: fc.MinLanes,
		Admit:    b.cellAdmission(fc.Orchestrator, iteration),
		Freshen:  b.wave().Freshen(),
		Launch:   launch,
		Leases:   b.laneLeases(fc.Orchestrator),
		Claims:   b.wave().Claims(),
		Reap:     b.laneReap(),
		Publish:  b.deps.Signals.Emit,
		Halted:   b.poolHalted,
		Stage:    fc.Orchestrator.Stage,
		Window:   b.laneWindow(fc),
	}
}

func (b *loopBatchCoordinator) cellAdmission(oc policy.OrchestratorConfig, iteration int) orchestrator.Admit {
	graph, err := b.moduleGraph(oc, iteration)
	if err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: orchestrator: pool %d admits by the file rule; the module graph did not load: %v\n", iteration, err)
		return nil
	}
	return orchestrator.AdmitByCell(admission.Admitter{Graph: graph, Text: b.wave().InboxText(iteration),
		OnlySharedEdits: oc.OnlySharedEdits(), Shadow: orchestrator.ReachShadow(b.deps.Signals.Emit)})
}

func (b *loopBatchCoordinator) orchestratedRun(fc policy.FleetConfig, launch fleet.LaunchFn, iteration int) poolRunFn {
	return func(ctx context.Context, backlog []fleet.Todo, resupply poolSupplyFn) []fleet.Result {
		pool := b.orchestratorPool(fc, launch, iteration)
		pool.Resupply = b.orchestratorResupply(resupply, iteration)
		return pool.Run(ctx, backlog)
	}
}

func (b *loopBatchCoordinator) orchestratorResupply(resupply poolSupplyFn, iteration int) func([]fleet.Todo) []fleet.Todo {
	return func(held []fleet.Todo) []fleet.Todo {
		todos, err := resupply(held)
		if err != nil {
			fmt.Fprintf(b.stderr, "[loop] WARN: orchestrator: pool %d resupply: %v; the free slot waits for the next lane exit\n", iteration, err)
			return nil
		}
		return todos
	}
}

func (b *loopBatchCoordinator) laneWindow(fc policy.FleetConfig) *orchestrator.Window {
	oc := fc.Orchestrator
	sized := fc.Count
	if fc.Concurrency > 0 {
		sized = min(sized, fc.Concurrency)
	}
	w := orchestrator.NewWindow(orchestrator.WindowConfig{Start: oc.WindowStart, Min: oc.WindowMin, Sized: sized}, b.deps.Signals.Emit)
	fmt.Fprintf(b.stderr, "[loop] orchestrator: adaptive window (stage %s): %d of %d lane(s), min %d\n", oc.Stage, w.Size(), sized, oc.WindowMin)
	return w
}
