package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseoutputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// runCycleOutputs reports whether each completed phase left what a reviewer
// needs, and the cycle's reasoning-chain status. Every decision and read lives
// in internal/phaseoutputs, shared with the loop's post-cycle signal emitter.
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
	warn := func(m string) { fmt.Fprintf(stderr, "evolve cycle outputs: WARN: %s\n", m) }
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, warn)
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

	survey := phaseoutputs.Survey(completed, listing, catalogAwareResolver(projectRoot, warn))
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

// catalogAwareResolver resolves report names through the builtin+user catalog
// the contract gate and bridge use; a builtin-only lookup reports false gaps.
// It degrades loudly to builtin-only when the registry cannot load.
func catalogAwareResolver(projectRoot string, warn func(string)) phasecontract.Resolver {
	builtinCat, err := phasespec.Load(config.RegistryPath(projectRoot))
	if err != nil {
		warn(fmt.Sprintf("builtin registry load failed (%v); resolving builtin-only", err))
		return phasecontract.BuiltinResolver{}
	}
	// Demotion only hides a spec from the SELECT menu and Get still resolves it,
	// so sharing the one path keeps the vocabularies identical.
	userSpecs, discWarns := discoverUserSpecsClamped(projectRoot, cmdutil.NewPromptsLoader(projectRoot))
	catalog, mergeWarns := builtinCat.Merge(userSpecs)
	for _, w := range append(discWarns, mergeWarns...) {
		warn(w)
	}
	return phasecontract.NewCatalogResolver(catalog.Get)
}
