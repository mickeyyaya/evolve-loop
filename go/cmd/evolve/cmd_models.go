package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	evolog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

func runModels(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve models: missing subcommand (refresh|list|performance)")
		return 10
	}
	switch args[0] {
	case "refresh":
		return runModelsRefresh(args[1:], stdout, stderr)
	case "list":
		return runModelsList(args[1:], stdout, stderr)
	case "performance":
		return runModelsPerformance(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve models: unknown subcommand %q\n", args[0])
		return 10
	}
}

type modelsOpts struct {
	EvolveDir, Project, Plugin, Adapters string
	AsJSON                               bool
	Source                               string
}

func parseModelsFlags(name string, args []string, stderr io.Writer) (modelsOpts, bool) {
	fs := flag.NewFlagSet("evolve models "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o modelsOpts
	var evolveDirFlag, projectRootFlag string
	fs.BoolVar(&o.AsJSON, "json", false, "emit as JSON instead of a human table")
	fs.StringVar(&o.Source, "source", "live", "refresh source: live (query /model) | detect (manifest map)")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.StringVar(&projectRootFlag, "project-root", "", "project root (default $EVOLVE_PROJECT_ROOT or cwd)")
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
	if o.Source != "live" && o.Source != "" && o.Source != "detect" {
		fmt.Fprintf(stderr, "evolve models refresh: unknown --source %q (live|detect)\n", o.Source)
		return 10
	}
	ctx := context.Background()
	rep := setup.Detect(ctx, setup.DetectOptions{
		ProjectRoot: o.Project, EvolveDir: o.EvolveDir, PluginRoot: o.Plugin, AdaptersDir: o.Adapters,
	})
	if o.Source == "detect" {
		return commitAndReport(o, detectRefresh(rep), "setup detect", stdout, stderr)
	}
	prior, _ := modelcatalog.Read(o.EvolveDir)
	cat, err := liveRefresh(ctx, rep, o.Project, o.EvolveDir, prior, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve models refresh: %v\n", err)
		return 1
	}
	return finishLiveRefresh(o, cat, stdout, stderr)
}

func finishLiveRefresh(o modelsOpts, cat modelcatalog.Catalog, stdout, stderr io.Writer) int {
	live, fallback := countLiveAndFallback(cat)
	if live > 0 {
		return commitAndReport(o, cat, fmt.Sprintf("live /model; %d live, %d detect fallback", live, fallback), stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve models refresh: no ready CLI was classified live (%d detect fallback); the catalog on disk is unchanged — each CLI's fallback reason is listed\n", fallback)
	if o.AsJSON {
		emitCatalogJSON(cat, stdout, stderr)
		return 1
	}
	fmt.Fprintf(stdout, "Model catalog NOT refreshed (source: live /model) → 0 of %d CLI(s) classified live:\n", len(cat.CLIs))
	printCatalogHuman(stdout, cat)
	return 1
}

func countLiveAndFallback(cat modelcatalog.Catalog) (live, fallback int) {
	live = cat.LiveCount()
	return live, len(cat.CLIs) - live
}

func commitAndReport(o modelsOpts, cat modelcatalog.Catalog, srcLbl string, stdout, stderr io.Writer) int {
	cat, err := modelcatalog.Commit(o.EvolveDir, cat, stderr)
	if err != nil {
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
		e := cat.CLIs[cli]
		tm := e.TierModels
		fmt.Fprintf(w, "  %-8s fast=%-16s balanced=%-16s deep=%s\n",
			cli, dash(tm["fast"]), dash(tm["balanced"]), dash(tm["deep"]))
		fmt.Fprintf(w, "  %-8s source: %s\n", "", sourceLabel(e))
		if len(e.Efforts) > 0 {
			fmt.Fprintf(w, "  %-8s efforts: %s\n", "", strings.Join(e.Efforts, ", "))
		}
	}
}

func sourceLabel(e modelcatalog.CLIEntry) string {
	if e.FallbackReason != "" {
		return "detect fallback, reason=" + evolog.DiagnosticField(e.FallbackReason)
	}
	return dash(e.Source)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func sortedCLIs(cat modelcatalog.Catalog) []string {
	out := make([]string, 0, len(cat.CLIs))
	for k := range cat.CLIs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
