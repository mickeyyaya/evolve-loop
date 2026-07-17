package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

// runModels implements `evolve models <subcommand>` — the live tier→model
// catalog cached at .evolve/model-catalog.json (Step 10 of the unified-config
// refactor). Subcommands:
//
//	refresh [--evolve-dir P] [--project-root P] [--json]   re-query CLIs, rewrite the cache
//	list    [--evolve-dir P] [--project-root P] [--json]   print the cached catalog + staleness
//
// Exit codes: 0 OK, 1 runtime error, 10 bad args.
//
// NOTE (Step 10b): refresh currently sources tier→model from `setup detect`
// (the manifest-derived map). The higher-fidelity live `/model`-query source
// is a flagged follow-up (10b-live) — it produces the same Catalog shape, so
// only the producer changes, not this command or the cache schema.
func runModels(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve models: missing subcommand (refresh|list)")
		return 10
	}
	switch args[0] {
	case "refresh":
		return runModelsRefresh(args[1:], stdout, stderr)
	case "list":
		return runModelsList(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve models: unknown subcommand %q\n", args[0])
		return 10
	}
}

// modelsOpts holds the parsed flags + resolved roots shared by both
// subcommands. `list` reads only EvolveDir/AsJSON; `refresh` also uses Source
// and the roots.
type modelsOpts struct {
	EvolveDir, Project, Plugin, Adapters string
	AsJSON                               bool
	// Source selects the refresh strategy: "live" queries each CLI's /model
	// (authoritative) with per-CLI detect fallback; "detect" uses only the
	// manifest-derived map (fast, no REPL driving). Ignored by `list`.
	Source string
}

// parseModelsFlags parses the shared flags and resolves the .evolve directory
// the same way `evolve setup` does.
func parseModelsFlags(name string, args []string, stderr io.Writer) (modelsOpts, bool) {
	fs := flag.NewFlagSet("evolve models "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o modelsOpts
	var evolveDirFlag, projectRootFlag string
	fs.BoolVar(&o.AsJSON, "json", false, "emit as JSON instead of a human table")
	fs.StringVar(&o.Source, "source", "live", "refresh source: live (query /model) | detect (manifest map)")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.StringVar(&projectRootFlag, "project-root", "", "project root (default $EVOLVE_PROJECT_ROOT or cwd)")
	// NOTE: pass args straight to Parse — do NOT route through reorderArgs.
	// reorderArgs is value-flag-unaware: it moves a value-taking flag's value
	// ("--evolve-dir /tmp" → "/tmp") to the end as a positional, leaving
	// "--evolve-dir" to swallow the next flag. These subcommands take no
	// positionals, so native flag parsing is both correct and sufficient.
	if err := fs.Parse(args); err != nil {
		return modelsOpts{}, false
	}
	o.Project, o.Plugin, o.EvolveDir, o.Adapters = setupRoots(projectRootFlag, evolveDirFlag, stderr)
	return o, true
}

func runModelsRefresh(args []string, stdout, stderr io.Writer) int {
	o, ok := parseModelsFlags("refresh", args, stderr)
	if !ok {
		return 10
	}
	ctx := context.Background()
	rep := setup.Detect(ctx, setup.DetectOptions{
		ProjectRoot: o.Project, EvolveDir: o.EvolveDir, PluginRoot: o.Plugin, AdaptersDir: o.Adapters,
	})

	var (
		cat    modelcatalog.Catalog
		err    error
		srcLbl string
	)
	switch o.Source {
	case "detect":
		cat, srcLbl = detectRefresh(rep), "setup detect"
	case "live", "":
		srcLbl = "live /model (detect fallback)"
		cat, err = liveRefresh(ctx, rep, o.Project, stderr)
	default:
		fmt.Fprintf(stderr, "evolve models refresh: unknown --source %q (live|detect)\n", o.Source)
		return 10
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve models refresh: %v\n", err)
		return 1
	}
	// Preserve operator-authored tier_fallbacks: the refresh rebuilds the
	// catalog wholesale, so merge prior chains in before overwriting the cache.
	// A corrupt prior cache must not block refresh — refresh IS the repair
	// path — so warn and write the unmerged catalog (no chains to preserve).
	if prior, readErr := modelcatalog.Read(o.EvolveDir); readErr != nil {
		fmt.Fprintf(stderr, "evolve models refresh: prior catalog unreadable, tier_fallbacks not preserved: %v\n", readErr)
	} else {
		cat = modelcatalog.MergeFallbacks(prior, cat)
	}
	if err := modelcatalog.Write(o.EvolveDir, cat); err != nil {
		fmt.Fprintf(stderr, "evolve models refresh: %v\n", err)
		return 1
	}
	if o.AsJSON {
		return emitCatalogJSON(cat, stdout, stderr)
	}
	fmt.Fprintf(stdout, "Refreshed model catalog (source: %s) → %d CLI(s):\n", srcLbl, len(cat.CLIs))
	printCatalogHuman(stdout, cat)
	return 0
}

// detectRefresh builds a catalog from setup.Detect's manifest-derived tier
// models (Source defaults to "detect" in BuildFromSnapshots — informational,
// not dispatch-authoritative).
func detectRefresh(rep setup.DetectReport) modelcatalog.Catalog {
	snaps := make([]modelcatalog.CLISnapshot, 0, len(rep.CLIs))
	for _, c := range rep.CLIs {
		snaps = append(snaps, modelcatalog.CLISnapshot{
			CLI: c.CLI, Ready: c.Verdict == "ready", TierModels: c.TierModels,
		})
	}
	return modelcatalog.BuildFromSnapshots(snaps, time.Now().UTC())
}

func runModelsList(args []string, stdout, stderr io.Writer) int {
	o, ok := parseModelsFlags("list", args, stderr)
	if !ok {
		return 10
	}
	cat, err := modelcatalog.Read(o.EvolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve models list: %v\n", err)
		return 1
	}
	// Empty-cache guard BEFORE the JSON branch so `list --json` on a fresh
	// repo emits an explicit signal rather than a zero-value {"clis":null}.
	if cat.Empty() {
		if o.AsJSON {
			fmt.Fprintln(stdout, `{"clis":{},"note":"empty — run: evolve models refresh"}`)
		} else {
			fmt.Fprintln(stdout, "No model catalog yet. Run: evolve models refresh")
		}
		return 0
	}
	if o.AsJSON {
		return emitCatalogJSON(cat, stdout, stderr)
	}
	stale := ""
	if cat.IsStale(time.Now(), modelcatalog.DefaultTTL) {
		stale = "  (STALE — older than 1 day; will refresh at next cycle start)"
	}
	fmt.Fprintf(stdout, "Model catalog (fetched %s)%s\n", cat.FetchedAt.Format(time.RFC3339), stale)
	printCatalogHuman(stdout, cat)
	return 0
}

func emitCatalogJSON(cat modelcatalog.Catalog, stdout, stderr io.Writer) int {
	buf, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "evolve models: %v\n", err)
		return 1
	}
	_, _ = stdout.Write(append(buf, '\n'))
	return 0
}

func printCatalogHuman(w io.Writer, cat modelcatalog.Catalog) {
	for _, cli := range sortedCLIs(cat) {
		tm := cat.CLIs[cli].TierModels
		fmt.Fprintf(w, "  %-8s fast=%-16s balanced=%-16s deep=%s\n",
			cli, dash(tm["fast"]), dash(tm["balanced"]), dash(tm["deep"]))
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// sortedCLIs returns the catalog's CLI keys in stable order for deterministic
// human output (map iteration order is randomized).
func sortedCLIs(cat modelcatalog.Catalog) []string {
	out := make([]string, 0, len(cat.CLIs))
	for k := range cat.CLIs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
