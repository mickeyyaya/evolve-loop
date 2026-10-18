package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type graphLoad struct {
	iteration int
	graph     *admission.ModuleGraph
	err       error
	loaded    bool
}

type graphLoader func(ctx context.Context, projectRoot string, hubMin int) (*admission.ModuleGraph, error)

func (b *loopBatchCoordinator) moduleGraph(oc policy.OrchestratorConfig, iteration int) (*admission.ModuleGraph, error) {
	if g := b.lastGraph; g.loaded && g.iteration == iteration {
		return g.graph, g.err
	}
	load := b.loadGraph
	if load == nil {
		load = loadModuleGraph
	}
	graph, err := load(b.ctx, b.cfg.ProjectRoot, oc.HubMinImporters)
	b.lastGraph = graphLoad{iteration: iteration, graph: graph, err: err, loaded: true}
	return graph, err
}

func (b *loopBatchCoordinator) takeSupply(iteration int, fc policy.FleetConfig) {
	if !fleetPlanRuns(fc) {
		return
	}
	graph, err := b.moduleGraph(fc.Orchestrator, iteration)
	if err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: orchestrator: census %d counts without cells; the module graph did not load: %v\n", iteration, err)
	}
	census := supplyWiring{root: b.cfg.ProjectRoot, evolveDir: b.cfg.EvolveDir, fleet: fc, graph: censusGraph(graph, err), iteration: iteration, now: time.Now, stderr: b.stderr}.census()
	census.Publish = b.deps.Signals.Emit
	if _, err := census.Take(); err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: orchestrator: census %d: %v\n", iteration, err)
	}
}
