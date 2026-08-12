package main

// `evolve cycle outputs [N]` — the read-only per-phase output accountant: for
// every phase the cycle completed, did the workspace end up holding the data a
// reviewer needs (report, prompt, events, usage), and what is the cycle's
// reasoning-chain status under the one-meaning-per-state totalization?
//
// Thin adapter by design: ALL decisions live in internal/phaseoutputs (pure,
// exhaustively tested) and ALL workspace reading goes through that package's
// shared loaders — the same ones the loop's post-cycle signal emitter uses, so
// the CLI and the unified signal stream cannot read the cycle differently.
// Flags mirror `evolve cycle timing` — same defaults, same resolution helpers.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseoutputs"
)

func runCycleOutputs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle outputs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		jsonOut     bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.BoolVar(&jsonOut, "json", false, "emit the survey as JSON")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle outputs: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	runsDir := filepath.Join(evolveDir, "runs")

	workspace, cycleLabel := resolveCycleWorkspace(runsDir, fs.Args())
	if workspace == "" {
		fmt.Fprintf(stderr, "evolve cycle outputs: no cycle workspace under %s\n", runsDir)
		return 10
	}

	completed, err := phaseoutputs.LoadCompletedPhases(workspace)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle outputs: %v\n", err)
		return 10
	}
	listing, err := phaseoutputs.LoadListing(workspace)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle outputs: list %s: %v\n", workspace, err)
		return 10
	}

	survey := phaseoutputs.Survey(completed, listing)
	reading := phaseoutputs.LoadShadowReading(workspace, auditchain.ShadowRecordFile)
	chain := phaseoutputs.CycleChainStatus(slices.Contains(completed, "audit"), reading)

	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Cycle string             `json:"cycle"`
			Chain string             `json:"chain"`
			Gaps  []string           `json:"gaps"`
			Rows  []phaseoutputs.Row `json:"rows"`
		}{cycleLabel, string(chain), survey.Gaps(), survey.Rows}); err != nil {
			fmt.Fprintf(stderr, "evolve cycle outputs: encode: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "cycle %s — %s\n", cycleLabel, survey.SummaryLine())
	fmt.Fprintf(stdout, "chain: %s\n", chain)
	return 0
}
