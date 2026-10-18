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

func loadModuleGraph(ctx context.Context, projectRoot string, hubMin int) (*admission.ModuleGraph, error) {
	bounded, cancel := context.WithTimeout(ctx, admissionGraphTimeout)
	defer cancel()
	return admission.LoadModuleGraph(bounded, sysexec.DefaultRunner, filepath.Join(projectRoot, "go"), hubMin)
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
		Inbox:     inboxmover.Options{InboxDir: filepath.Join(w.evolveDir, "inbox"), Stderr: io.Discard},
		Rank:      rank,
		Protected: laneForbidden(w.root, w.stderr),
		Graph:     w.graph,
		Text:      engine.InboxText(w.iteration),
		Window:    window,
		Path:      waveSupplyPath(w.evolveDir),
		Now:       w.now,
	}
}

func runWaveSupply(env waveEnv, args []string, stdout, stderr io.Writer) int {
	projectRoot, asJSON, err := parseWaveReadArgs(args)
	if err != nil {
		return waveUsageError(stderr, err)
	}
	plane, err := resolveWavePlane(env, projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	fc := loadFleetConfig(plane.evolveDir, stderr, nilSignals())
	graph, err := env.loadGraph(context.Background(), plane.root, fc.Orchestrator.HubMinImporters)
	if err != nil {
		fmt.Fprintf(stderr, "%sWARN: the module graph did not load: %v\n", wavePrefix, err)
	}
	wiring := supplyWiring{root: plane.root, evolveDir: plane.evolveDir, fleet: fc, graph: censusGraph(graph, err), now: env.now, stderr: stderr}
	supply, err := wiring.census().Count()
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	if asJSON {
		return writeWaveJSON(supply, stdout, stderr)
	}
	writeWaveSupplyText(stdout, supply)
	return 0
}

func writeWaveSupplyText(w io.Writer, s orchestrator.Supply) {
	fmt.Fprintf(w, "supply: %d admissible of %d item(s), window %d\n", s.Counts[orchestrator.ClassAdmissible], s.Total(), s.Window)
	for _, class := range orchestrator.Classes() {
		fmt.Fprintf(w, "  %-18s %d\n", class, s.Counts[class])
	}
	if len(s.Refusals) > 0 {
		fmt.Fprintln(w, "refused:")
	}
	for _, r := range s.Refusals {
		fmt.Fprintf(w, "  %s  %s: %s\n", r.Class, r.ID, r.Reason)
	}
	if len(s.Proposals) > 0 {
		fmt.Fprintln(w, "split proposals:")
	}
	for _, p := range s.Proposals {
		fmt.Fprintf(w, "  %s -> %s + %s\n", p.Item, p.Console, p.Lane)
	}
	if len(s.Warnings) > 0 {
		fmt.Fprintln(w, "warnings:")
	}
	for _, warning := range s.Warnings {
		fmt.Fprintf(w, "  %s\n", warning)
	}
}
