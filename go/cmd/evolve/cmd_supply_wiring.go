package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const admissionGraphTimeout = 30 * time.Second

type supplyWiring struct {
	root, evolveDir string
	fleet           policy.FleetConfig
	graph           admission.Graph
	iteration       int
	now             func() time.Time
	stderr          io.Writer
}

func waveSupplyPath(evolveDir string) string {
	return filepath.Join(evolveDir, "waves", orchestrator.SupplyFile)
}

func graphLoaderOver(run sysexec.RunFunc) graphLoader {
	return func(ctx context.Context, projectRoot string, hubMin int) (*admission.ModuleGraph, error) {
		bounded, cancel := context.WithTimeout(ctx, admissionGraphTimeout)
		defer cancel()
		return admission.LoadModuleGraph(bounded, run, filepath.Join(projectRoot, "go"), hubMin)
	}
}

func loadModuleGraph(ctx context.Context, projectRoot string, hubMin int) (*admission.ModuleGraph, error) {
	return graphLoaderOver(sysexec.DefaultRunner)(ctx, projectRoot, hubMin)
}

func censusGraph(graph *admission.ModuleGraph, err error) admission.Graph {
	if err != nil {
		return nil
	}
	return graph
}

func (w supplyWiring) census() orchestrator.Census {
	rank, warnings := rankinputs.Load(w.evolveDir, w.now())
	for _, warning := range warnings {
		fmt.Fprintf(w.stderr, "[census] WARN: %s\n", warning)
	}
	engine := nullWaveEngine(loopwave.RootsOf(w.root), w.stderr)
	window, _ := engine.QuotaCount(w.fleet, loopwave.LoadRouting(w.evolveDir, w.stderr))
	return orchestrator.Census{
		Inbox:           inboxmover.Options{InboxDir: filepath.Join(w.evolveDir, "inbox"), Stderr: io.Discard},
		Rank:            rank,
		Protected:       laneForbidden(w.root, w.stderr),
		Graph:           w.graph,
		OnlySharedEdits: w.fleet.Orchestrator.OnlySharedEdits(),
		Text:            engine.InboxText(w.iteration),
		Window:          window,
		Path:            waveSupplyPath(w.evolveDir),
		Now:             w.now,
	}
}
