package main

// `evolve setup latest` — the read-only live-latest probe behind /evo:setup
// (operator directive): query EVERY ready LLM CLI's bridge IN PARALLEL for the
// models it currently offers, and report per family whether a model fresher
// than today's dispatch map exists in the same lineage. No writes — adopting
// the latest is `evolve models refresh` (catalog commit, policy-family-gated),
// and this probe is the staleness evidence the setup skill offers it on.
//
// Thin adapter: the staleness decision is setup.ComputeLatest (pure); the
// fan-out here only sequences captures and preserves detection order.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

// setupLatestProbeTimeout bounds ONE family's live capture; the fan-out is
// parallel, so the whole probe costs roughly the slowest single capture.
// Var, not const: the timeout wiring is test-pinned (a dropped timeout let a
// hung capture stall the whole probe).
var setupLatestProbeTimeout = 90 * time.Second

// latestProbeTiers is the fixed tier order staleness is judged over — the
// WHOLE dispatch map, not only deep (most phases dispatch balanced).
var latestProbeTiers = []string{"fast", "balanced", "deep", "top"}

// setupLatestReport fans the live listing out across every READY family and
// folds the results into detection order. catTiers is the catalog's tier map
// (the hot-reloading dispatch authority) — when it knows a family's tier,
// staleness is judged against it, not the manifest default. A per-family
// probe failure stays on its own row; siblings are unaffected.
func setupLatestReport(ctx context.Context, rep setup.DetectReport, catTiers map[string]map[string]string, lister modelquery.Lister, fresh map[string]modelquery.FreshnessPolicy) setup.LatestReport {
	var ready []setup.CLIStatus
	for _, c := range rep.CLIs {
		if c.Verdict == "ready" {
			ready = append(ready, c)
		}
	}
	rows := make([]setup.FamilyLatest, len(ready))
	var wg sync.WaitGroup
	for i, c := range ready {
		wg.Add(1)
		go func(i int, c setup.CLIStatus) {
			defer wg.Done()
			tierModel := func(tier string) string {
				if cat, ok := catTiers[c.CLI]; ok && cat[tier] != "" {
					return cat[tier]
				}
				return c.TierModels[tier]
			}
			row := setup.FamilyLatest{CLI: c.CLI, CurrentDeepModel: tierModel("deep")}
			cctx, cancel := context.WithTimeout(ctx, setupLatestProbeTimeout)
			defer cancel()
			ids, err := lister.List(cctx, c.CLI)
			if err != nil {
				row.Error = err.Error()
				rows[i] = row
				return
			}
			row.Candidates = len(ids)
			for _, tier := range latestProbeTiers {
				current := tierModel(tier)
				if current == "" || current == tier {
					// Absent, or the identity fallback (a tier WORD, not a
					// model) — nothing judgeable; never fabricate a verdict.
					continue
				}
				latest, stale, observed := setup.ComputeLatest(current, ids, fresh[c.CLI])
				if tier == "deep" {
					row.LatestModel, row.CurrentSeenLive = latest, observed
				}
				switch {
				case stale: // stale ⇒ observed structurally (an unseen model can never differ from its own singleton bucket)
					row.StaleTiers = append(row.StaleTiers, setup.TierStale{Tier: tier, Current: current, Latest: latest})
				case !observed:
					row.UnverifiedTiers = append(row.UnverifiedTiers, tier)
				}
			}
			row.MapStale = len(row.StaleTiers) > 0
			rows[i] = row
		}(i, c)
	}
	wg.Wait()
	return setup.LatestReport{Source: "live", CLIs: rows}
}

// perCLIScratchLister gives each family its own scratch workspace so parallel
// tmux captures cannot collide on pane/log filenames, salvaging each family's
// probe diagnostics before teardown (a deleted failed-probe artifact explains
// nothing — Rule 12).
type perCLIScratchLister struct {
	evolveDir string
	log       io.Writer
}

func (l perCLIScratchLister) List(ctx context.Context, cli string) ([]string, error) {
	scratch, err := os.MkdirTemp("", "evolve-setup-latest-"+cli+"-*")
	if err != nil {
		return nil, fmt.Errorf("scratch workspace: %w", err)
	}
	defer func() {
		salvageProbeDiagnostics(scratch, l.evolveDir, cli, time.Now, l.log)
		_ = os.RemoveAll(scratch)
	}()
	router := modelquery.Router{
		ByCLI:   map[string]modelquery.Lister{"ollama": modelquery.OllamaLister{}},
		Default: modelquery.RecipeLister{Capturer: bridgeModelCapturer{workspace: scratch}},
	}
	return router.List(ctx, cli)
}

func runSetupLatest(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve setup latest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	var evolveDirFlag, projectRootFlag string
	fs.BoolVar(&asJSON, "json", false, "emit the probe as JSON (default human table)")
	fs.StringVar(&evolveDirFlag, "evolve-dir", "", "path to .evolve/ (default <project>/.evolve)")
	fs.StringVar(&projectRootFlag, "project-root", "", "project root (default $EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	project, plugin, evolveDir, adapters := setupRoots(projectRootFlag, evolveDirFlag, stderr)
	ctx := context.Background()
	rep := setup.Detect(ctx, setup.DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir, PluginRoot: plugin, AdaptersDir: adapters,
	})
	var readyCLIs []string
	for _, c := range rep.CLIs {
		if c.Verdict == "ready" {
			readyCLIs = append(readyCLIs, c.CLI)
		}
	}
	// Catalog read is tolerant: no catalog just means the manifest map is the
	// staleness baseline (exactly what dispatch would fall back to).
	catTiers := map[string]map[string]string{}
	if cat, err := modelcatalog.Read(evolveDir); err == nil {
		for name, c := range cat.CLIs {
			if len(c.TierModels) > 0 {
				catTiers[name] = c.TierModels
			}
		}
	}
	report := setupLatestReport(ctx, rep, catTiers,
		perCLIScratchLister{evolveDir: evolveDir, log: stderr},
		freshnessFromManifests(readyCLIs))

	if asJSON {
		buf, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve setup latest: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", buf)
		return 0
	}
	fmt.Fprintf(stdout, "Live latest-model probe (%d ready CLI(s), parallel):\n", len(report.CLIs))
	for _, r := range report.CLIs {
		switch {
		case r.Error != "":
			fmt.Fprintf(stdout, "  %-8s probe FAILED: %s\n", r.CLI, r.Error)
		case r.MapStale:
			pairs := make([]string, 0, len(r.StaleTiers))
			for _, st := range r.StaleTiers {
				pairs = append(pairs, fmt.Sprintf("%s %s→%s", st.Tier, st.Current, st.Latest))
			}
			fmt.Fprintf(stdout, "  %-8s STALE: %s (%d live models) — `evolve models refresh` adopts it\n",
				r.CLI, strings.Join(pairs, ", "), r.Candidates)
		case !r.CurrentSeenLive:
			fmt.Fprintf(stdout, "  %-8s UNVERIFIED: %s was not among the %d live models — the map needs manual verification, freshness cannot be judged\n",
				r.CLI, r.CurrentDeepModel, r.Candidates)
		default:
			fmt.Fprintf(stdout, "  %-8s current: %s is the freshest in its line (%d live models)\n",
				r.CLI, r.CurrentDeepModel, r.Candidates)
		}
	}
	return 0
}
