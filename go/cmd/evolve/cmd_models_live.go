package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	evolog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func shouldRefreshCatalog(cat modelcatalog.Catalog, now time.Time, autoRefresh bool) bool {
	if !autoRefresh {
		return false
	}
	return cat.IsStale(now, modelcatalog.DefaultTTL)
}

func makeCatalogRefresher(projectRoot, evolveDir, stage string, emit signalcenter.Listener) func(context.Context) error {
	return func(ctx context.Context) error {
		return runStagedCatalogRefresh(ctx, evolveDir, stage, func(ctx context.Context, prior modelcatalog.Catalog) (modelcatalog.Catalog, error) {
			plugin := os.Getenv("EVOLVE_PLUGIN_ROOT")
			if plugin == "" {
				plugin = projectRoot
			}
			rep := setup.Detect(ctx, setup.DetectOptions{
				ProjectRoot: projectRoot, EvolveDir: evolveDir,
				PluginRoot: plugin, AdaptersDir: filepath.Join(plugin, "adapters"),
			})
			return liveRefresh(ctx, rep, projectRoot, evolveDir, prior, os.Stderr)
		}, time.Now, os.Stderr, emit)
	}
}

func runStagedCatalogRefresh(ctx context.Context, evolveDir, stage string, refresh func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error), now func() time.Time, log io.Writer, emit signalcenter.Listener) error {
	if stage != "shadow" && stage != "enforce" {
		return nil
	}
	live, rerr := modelcatalog.Read(evolveDir)
	if rerr != nil {
		fmt.Fprintf(log, "[models] WARN unreadable catalog (will refresh): %v\n", rerr)
	}
	gate := live
	if stage == "shadow" {
		shadow, serr := modelcatalog.ReadShadow(evolveDir)
		if serr != nil {
			fmt.Fprintf(log, "[models] WARN unreadable shadow catalog (will refresh): %v\n", serr)
		}
		gate = shadow
	}
	if !shouldRefreshCatalog(gate, now(), true) {
		return nil
	}
	fresh, err := refresh(ctx, gate)
	if err != nil {
		return err
	}
	if fresh.LiveCount() == 0 {
		fmt.Fprintf(log, "[models] WARN no ready CLI was classified live; the %s catalog and its fetched_at stay as they were, so the next cycle start retries\n", stage)
		return nil
	}
	if stage == "shadow" {
		for _, line := range shadowDiffLines(live, fresh) {
			fmt.Fprintf(log, "[models] shadow %s\n", line)
		}
		return modelcatalog.WriteShadow(evolveDir, fresh)
	}
	return commitKeepingLivePicks(evolveDir, live, fresh, log, emit)
}

func commitKeepingLivePicks(evolveDir string, live, fresh modelcatalog.Catalog, log io.Writer, emit signalcenter.Listener) error {
	merged, kept := modelcatalog.KeepLivePicks(live, fresh)
	for _, cli := range kept {
		fmt.Fprintf(log, "[models] WARN catalog_refresh_degraded %s: the live list failed (%s); the prior live pick stays\n", cli, fresh.CLIs[cli].FallbackReason)
		emit(modelcatalog.RefreshDegradedEvent("commitKeepingLivePicks", cli, fresh.CLIs[cli].FallbackReason))
	}
	_, cerr := modelcatalog.Commit(evolveDir, merged, log)
	return cerr
}

func shadowDiffLines(live, next modelcatalog.Catalog) []string {
	names := make(map[string]bool, len(live.CLIs)+len(next.CLIs))
	for cli := range live.CLIs {
		names[cli] = true
	}
	for cli := range next.CLIs {
		names[cli] = true
	}
	sorted := make([]string, 0, len(names))
	for cli := range names {
		sorted = append(sorted, cli)
	}
	sort.Strings(sorted)
	orNone := func(s string) string {
		if s == "" {
			return "(none)"
		}
		return s
	}
	var lines []string
	for _, cli := range sorted {
		for _, tier := range modelcatalog.CanonicalTiers {
			from := live.CLIs[cli].TierModels[tier]
			to := next.CLIs[cli].TierModels[tier]
			if from != to {
				lines = append(lines, fmt.Sprintf("would-change %s.%s: %s -> %s", cli, tier, orNone(from), orNone(to)))
			}
		}
	}
	return lines
}

type bridgeModelCapturer struct {
	workspace string
	deps      bridge.Deps
}

func (c bridgeModelCapturer) CaptureModelPicker(ctx context.Context, cli string) (string, error) {
	r, err := bridge.Resolve(driverSet, cli)
	if err != nil {
		return "", err
	}
	driver := r.Driver
	cfg := &bridge.Config{
		CLI:         driver,
		Workspace:   c.workspace,
		Agent:       "models",
		Realization: bridge.RealizeFor(driver, bridge.LaunchIntent{}),
	}
	return bridge.CaptureModelPicker(ctx, cfg, c.deps, driver)
}

const classifierProfileFile = "model-classifier.json"

type bridgePromptDispatcher struct {
	workspace   string
	projectRoot string
	launch      func(context.Context, core.BridgeRequest) (core.BridgeResponse, error)
}

func (d bridgePromptDispatcher) DispatchPrompt(ctx context.Context, cli, prompt string) (string, error) {
	req := d.request(cli, prompt)
	if err := os.Remove(req.ArtifactPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("bridgePromptDispatcher: clear %s's previous reply: %w", req.CLI, err)
	}
	resp, err := d.launcher()(ctx, req)
	if err != nil {
		return "", fmt.Errorf("bridgePromptDispatcher: launch %s: %w", req.CLI, err)
	}
	return resp.Stdout, nil
}

func (d bridgePromptDispatcher) launcher() func(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	if d.launch != nil {
		return d.launch
	}
	return bridge.NewEngine(bridge.Deps{TokenResolver: tokenusage.HostResolver(nil)}).Launch
}

func (d bridgePromptDispatcher) request(cli, prompt string) core.BridgeRequest {
	artifact := filepath.Join(d.workspace, "model-classifier-artifact.txt")
	return core.BridgeRequest{
		CLI:          cli,
		Profile:      filepath.Join(d.projectRoot, ".evolve", "profiles", classifierProfileFile),
		Prompt:       prompt + fmt.Sprintf("\n\nWrite that JSON object, and nothing else, to the file %s, then reply with the same JSON.\n", artifact),
		Workspace:    d.workspace,
		ProjectRoot:  d.projectRoot,
		Agent:        "model-classifier",
		ArtifactPath: artifact,
		Completion:   core.CompletionArtifact,
	}
}

func liveRefresh(ctx context.Context, rep setup.DetectReport, projectRoot, evolveDir string, prior modelcatalog.Catalog, log io.Writer) (modelcatalog.Catalog, error) {
	projectRoot, evolveDir, err := liveRefreshRoots(projectRoot, evolveDir)
	if err != nil {
		return modelcatalog.Catalog{}, err
	}
	readyCLIs, fallback := readyCLIsWithDetectMaps(rep)
	if len(readyCLIs) == 0 {
		return modelcatalog.Catalog{}, fmt.Errorf("no ready CLIs to query (run: evolve setup detect)")
	}

	scratch, err := classifierScratch(projectRoot)
	if err != nil {
		return modelcatalog.Catalog{}, fmt.Errorf("liveRefresh: scratch workspace: %w", err)
	}
	defer func() {
		salvageProbeDiagnostics(scratch, evolveDir, "", time.Now, log)
		_ = os.RemoveAll(scratch)
	}()

	chain, err := classifierChain(projectRoot, readyCLIs, log)
	if err != nil {
		return modelcatalog.Catalog{}, err
	}
	dispatcher := bridgePromptDispatcher{workspace: scratch, projectRoot: projectRoot}
	return modelquery.Refresh(ctx, modelquery.RefreshDeps{
		CLIs:            readyCLIs,
		Lister:          modelquery.DefaultRouter(bridgeModelCapturer{workspace: scratch}),
		Classifier:      modelquery.ChainClassifier{CLIs: chain, Dispatcher: dispatcher, Log: log},
		Fallback:        fallback,
		AllowedFamilies: policyAllowedFamilies(projectRoot),
		EffortListers:   modelquery.DefaultEffortListers(),
		Prior:           prior,
		Freshness:       freshnessFromManifests(readyCLIs),
		Now:             time.Now,
		Log:             log,
	})
}

func liveRefreshRoots(projectRoot, evolveDir string) (string, string, error) {
	if projectRoot == "" {
		cwd, err := processGetwd()
		if err != nil {
			return "", "", fmt.Errorf("liveRefresh: resolve workspace: %w", err)
		}
		projectRoot = cwd
	}
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	return projectRoot, evolveDir, nil
}

func readyCLIsWithDetectMaps(rep setup.DetectReport) ([]string, map[string]map[string]string) {
	var ready []string
	detectMaps := make(map[string]map[string]string)
	for _, c := range rep.CLIs {
		if c.Verdict != "ready" {
			continue
		}
		ready = append(ready, c.CLI)
		if len(c.TierModels) > 0 {
			detectMaps[c.CLI] = c.TierModels
		}
	}
	return ready, detectMaps
}

func policyAllowedFamilies(projectRoot string) map[string][]string {
	return policy.LoadOrDefaults(filepath.Join(projectRoot, ".evolve", "policy.json"), os.Stderr).CatalogConfig().AllowedFamilies
}

func classifierChain(projectRoot string, ready []string, log io.Writer) ([]string, error) {
	drivers, err := classifierPreference(projectRoot, routingHost(log, time.Now))
	if err != nil {
		return nil, err
	}
	return pickClassifierCLI(ready, drivers), nil
}

func classifierPreference(projectRoot string, host cliroute.Host) ([]string, error) {
	router, _, err := loadCLIRouter(projectRoot, host)
	if err != nil {
		return nil, fmt.Errorf("liveRefresh: the CLI routing table refuses to route the model classifier: %w", err)
	}
	return classifierDrivers(router, projectRoot)
}

func classifierDrivers(router *cliroute.Router, projectRoot string) ([]string, error) {
	d, err := router.Resolve(cliroute.Request{Agent: cliroute.ClassifierAgent, Launch: cliroute.LaunchClassifier, ProjectRoot: projectRoot})
	if err != nil {
		return nil, fmt.Errorf("liveRefresh: no route for the model classifier: %w", err)
	}
	return slices.Clone(d.Plan.Candidates), nil
}

func classifierScratchParent(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "ephemeral", "model-classifier")
}

func classifierScratch(projectRoot string) (string, error) {
	parent := classifierScratchParent(projectRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, "probe-")
}

func salvageProbeDiagnostics(scratch, evolveDir, tag string, now func() time.Time, log io.Writer) {
	entries, err := os.ReadDir(scratch)
	if err != nil {
		return
	}
	durable := filepath.Join(evolveDir, "models-probe")
	ensured := false
	ensure := func() bool {
		if !ensured {
			if err := os.MkdirAll(durable, 0o755); err != nil {
				return false
			}
			ensured = true
		}
		return true
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		src := filepath.Join(scratch, name)
		switch {
		case name == "escalation-report.json":
			stamp := now().UTC().Format("20060102T150405Z")
			if tag != "" {
				stamp += "-" + tag
			}
			dst := fmt.Sprintf("escalation-report-%s.json", stamp)
			if raw, rerr := os.ReadFile(src); rerr == nil && ensure() {
				if werr := os.WriteFile(filepath.Join(durable, dst), raw, 0o644); werr == nil {
					fmt.Fprintf(log, "[models] WARN probe escalation salvaged destination=%s — a live /model probe needed operator attention\n",
						evolog.DiagnosticField(filepath.Join(durable, dst)))
				}
			}
		case strings.HasSuffix(name, "-launch-error.txt"):
			dstName := name
			if tag != "" {
				dstName = tag + "-" + name
			}
			if raw, rerr := os.ReadFile(src); rerr == nil && ensure() {
				if werr := os.WriteFile(filepath.Join(durable, dstName), raw, 0o644); werr == nil {
					fmt.Fprintf(log, "[models] WARN probe launch-error salvaged destination=%s\n",
						evolog.DiagnosticField(filepath.Join(durable, dstName)))
				}
			}
		case name == bridge.LLMCallsLogFilename:
			if ensure() {
				result, importErr := llmcalls.Import(filepath.Join(durable, name), src)
				if importErr != nil {
					fmt.Fprintf(log, "[models] WARN probe attempt-ledger salvage failed source=%s destination=%s detail=%s\n",
						evolog.DiagnosticField(src), evolog.DiagnosticField(filepath.Join(durable, name)),
						evolog.DiagnosticField(importErr.Error()))
				} else if result.Skipped > 0 {
					fmt.Fprintf(log, "[models] WARN probe attempt-ledger salvage skipped=%d malformed record(s) source=%s\n",
						result.Skipped, evolog.DiagnosticField(src))
				}
			}
		}
	}
}

func freshnessFromManifests(clis []string) map[string]modelquery.FreshnessPolicy {
	out := make(map[string]modelquery.FreshnessPolicy, len(clis))
	for _, cli := range clis {
		r, err := bridge.Resolve(driverSet, cli)
		if err != nil {
			continue
		}
		if r.Manifest.ModelFreshness.Prefer == "alias" {
			out[cli] = modelquery.FreshnessPolicy{PreferAlias: true, AliasIDs: r.Manifest.ModelFreshness.AliasIDs}
		}
	}
	return out
}

func pickClassifierCLI(ready, drivers []string) []string {
	readyFamily := make(map[string]bool, len(ready))
	for _, r := range ready {
		readyFamily[r] = true
	}
	order := make([]string, 0, len(drivers))
	for _, driver := range drivers {
		if readyFamily[llmroute.Family(driver)] {
			order = append(order, driver)
		}
	}
	return order
}
