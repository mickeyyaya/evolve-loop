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
)

// shouldRefreshCatalog is the pure cycle-start gate: refresh only when enabled
// AND the cached catalog is older than the TTL (so the live /model drive runs
// at most once per day, not every cycle). Reads autoRefresh from policy.CatalogConfig().
func shouldRefreshCatalog(cat modelcatalog.Catalog, now time.Time, autoRefresh bool) bool {
	if !autoRefresh {
		return false
	}
	return cat.IsStale(now, modelcatalog.DefaultTTL)
}

// makeCatalogRefresher returns the closure core.WithCatalogRefresher invokes
// at cycle start. stage is the resolved policy.CatalogConfig().RefreshStage:
// "off" is a no-op, "shadow" runs the full live pipeline but writes only the
// shadow catalog (dispatch byte-identical to off), "enforce" commits the live
// catalog. TTL-gated per stage and best-effort; a failure propagates to the
// orchestrator which only WARNs.
func makeCatalogRefresher(projectRoot, evolveDir, stage string) func(context.Context) error {
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
		}, time.Now, os.Stderr)
	}
}

// runStagedCatalogRefresh is the stage-aware refresh spine, separated from
// makeCatalogRefresher so every stage behavior is testable with a fake
// pipeline (no live CLI drive, no setup.Detect):
//
//   - off (and anything unresolved — policy already fail-safes unknowns): no-op.
//   - shadow: TTL-gate on the SHADOW catalog (gating on the live file would
//     either never run or drive the expensive live probe every cycle), refresh
//     with the shadow catalog as Prior (that is where the reuse fingerprints
//     this stage wrote live), emit per-tier would-change lines against the
//     live catalog, and write ONLY the shadow file.
//   - enforce: TTL-gate on the live catalog and Commit (the one write seam —
//     carries operator tier_fallbacks, rotates .prev).
func runStagedCatalogRefresh(ctx context.Context, evolveDir, stage string, refresh func(context.Context, modelcatalog.Catalog) (modelcatalog.Catalog, error), now func() time.Time, log io.Writer) error {
	if stage != "shadow" && stage != "enforce" {
		return nil
	}
	live, rerr := modelcatalog.Read(evolveDir)
	if rerr != nil {
		// Corrupt cache: treat as stale (refresh overwrites it) but surface
		// the corruption rather than papering over it silently.
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
	if stage == "shadow" {
		for _, line := range shadowDiffLines(live, fresh) {
			fmt.Fprintf(log, "[models] shadow %s\n", line)
		}
		return modelcatalog.WriteShadow(evolveDir, fresh)
	}
	// Same seam as `evolve models refresh`.
	_, cerr := modelcatalog.Commit(evolveDir, fresh, log)
	return cerr
}

// shadowDiffLines renders the per-CLI per-tier differences between the live
// catalog and a shadow refresh result — the soak evidence that earns (or
// blocks) the flip to enforce. Deterministic order: sorted CLI names ×
// CanonicalTiers. An absent side renders as "(none)".
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

// bridgeModelCapturer adapts bridge.CaptureModelPicker to
// modelquery.ModelCapturer. It translates a base CLI name (codex|agy|claude)
// to the tmux driver the bridge launches (codex-tmux, …).
type bridgeModelCapturer struct {
	workspace string
}

func (c bridgeModelCapturer) CaptureModelPicker(ctx context.Context, cli string) (string, error) {
	driver := cli + "-tmux"
	cfg := &bridge.Config{
		CLI:         driver,
		Workspace:   c.workspace,
		Agent:       "models",
		Realization: bridge.RealizeFor(driver, bridge.LaunchIntent{}),
	}
	return bridge.CaptureModelPicker(ctx, cfg, bridge.Deps{}, driver)
}

// bridgePromptDispatcher adapts bridge.Engine.Launch to
// modelquery.PromptDispatcher: the tier-classification prompt is dispatched
// through the same sandboxed, liveness-probed, cli_fallback-aware bridge
// every phase uses, instead of a raw exec. It
// translates a base CLI name (codex|agy|claude) to the headless driver the
// bridge launches (driver_codex.go/driver_agy.go/driver_claudep.go), mirroring
// bridgeModelCapturer's cli->driver translation for the tmux pickers.
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
	return bridge.NewEngine(bridge.Deps{}).Launch
}

func (d bridgePromptDispatcher) request(cli, prompt string) core.BridgeRequest {
	driver := cli
	if cli == "claude" {
		driver = "claude-p"
	}
	artifact := filepath.Join(d.workspace, "model-classifier-artifact.txt")
	return core.BridgeRequest{
		CLI:          driver,
		Profile:      filepath.Join(d.projectRoot, ".evolve", "profiles", "router.json"),
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

	// The probe (tmux picker capture + one-shot classifier) works in a
	// throwaway scratch dir, never the repo: the router profile's sandbox
	// declares read_only_repo, so an ArtifactPath under the project root is
	// either denied (artifact-timeout per CLI at cycle start) or litters an
	// untracked file in main. The profile path stays anchored to projectRoot.
	// Diagnostics the bridge writes under the workspace (escalation reports,
	// launch errors, the llm-calls token ledger) are salvaged to a durable
	// home BEFORE teardown — deleting them with the scratch dir would silently
	// destroy the one artifact that explains a failed probe.
	scratch, err := os.MkdirTemp("", "evolve-models-probe-*")
	if err != nil {
		return modelcatalog.Catalog{}, fmt.Errorf("liveRefresh: scratch workspace: %w", err)
	}
	defer func() {
		salvageProbeDiagnostics(scratch, evolveDir, "", time.Now, log)
		_ = os.RemoveAll(scratch)
	}()

	preference, err := classifierPreference(projectRoot)
	if err != nil {
		return modelcatalog.Catalog{}, err
	}
	dispatcher := bridgePromptDispatcher{workspace: scratch, projectRoot: projectRoot}
	return modelquery.Refresh(ctx, modelquery.RefreshDeps{
		CLIs:            readyCLIs,
		Lister:          modelquery.DefaultRouter(bridgeModelCapturer{workspace: scratch}),
		Classifier:      tierClassifier(readyCLIs, preference, dispatcher, log),
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
		cwd, err := os.Getwd()
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
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return nil
	}
	return pol.CatalogConfig().AllowedFamilies
}

func tierClassifier(ready, preference []string, dispatcher modelquery.PromptDispatcher, log io.Writer) modelquery.ChainClassifier {
	return modelquery.ChainClassifier{CLIs: pickClassifierCLI(ready, preference, ""), Dispatcher: dispatcher, Log: log}
}

func classifierPreference(projectRoot string) ([]string, error) {
	router, _, err := loadCLIRouter(projectRoot, cliroute.Host{})
	if err != nil {
		return nil, fmt.Errorf("liveRefresh: the CLI routing table refuses to route the model classifier: %w", err)
	}
	return classifierFamilies(router)
}

func classifierFamilies(router *cliroute.Router) ([]string, error) {
	d, err := router.Resolve(cliroute.Request{Agent: cliroute.ClassifierAgent, Launch: cliroute.LaunchClassifier})
	if err != nil {
		return nil, fmt.Errorf("liveRefresh: no route for the model classifier: %w", err)
	}
	var families []string
	for _, cli := range d.Plan.Candidates {
		if fam := llmroute.Family(cli); !slices.Contains(families, fam) {
			families = append(families, fam)
		}
	}
	return families, nil
}

// salvageProbeDiagnostics copies the diagnostic side-effects the bridge wrote
// under the scratch probe workspace into evolveDir/models-probe before the
// scratch dir is deleted:
//
//   - escalation-report.json → escalation-report-<UTC stamp>.json (each event
//     kept, never clobbered) + a WARN naming the salvaged path
//   - *-launch-error.txt → copied verbatim + WARN
//   - llm-calls.ndjson → APPENDED to the durable ledger, so the
//     token-telemetry trail keeps every classifier call the probe made
//
// Deliberately allowlist-shaped: artifacts/prompts/pane logs are probe
// plumbing and stay disposable. Best-effort throughout — salvage must never
// fail the refresh — and fully quiet when a clean probe left nothing behind.
// tag disambiguates destinations when multiple probes salvage concurrently
// into the shared durable home (`evolve setup latest` runs one salvage per
// family in parallel): a second-granularity stamp alone can still collide,
// and the un-stamped launch-error name collides outright since every
// family's capturer is Agent "models". Empty tag keeps the historical names
// (single-probe callers).
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

// freshnessFromManifests maps each ready CLI's manifest model_freshness block
// to a modelquery.FreshnessPolicy. This is the composition-root translation
// of a bridge-declared CLI fact into a plain value — modelquery never imports
// bridge, and the per-CLI difference lives in manifest DATA, not a Go
// conditional. A CLI with no manifest (ollama) or no block gets no entry ⇒
// zero policy (newest concrete version wins).
func freshnessFromManifests(clis []string) map[string]modelquery.FreshnessPolicy {
	out := make(map[string]modelquery.FreshnessPolicy, len(clis))
	for _, cli := range clis {
		m, err := bridge.LoadManifest(cli + "-tmux")
		if err != nil {
			continue
		}
		if m.ModelFreshness.Prefer == "alias" {
			out[cli] = modelquery.FreshnessPolicy{PreferAlias: true, AliasIDs: m.ModelFreshness.AliasIDs}
		}
	}
	return out
}

func pickClassifierCLI(ready, preference []string, overrideCLI string) []string {
	remaining := make(map[string]bool, len(ready))
	for _, r := range ready {
		remaining[r] = true
	}
	order := make([]string, 0, len(ready))
	take := func(cli string) {
		if remaining[cli] {
			order = append(order, cli)
			delete(remaining, cli)
		}
	}
	take(overrideCLI)
	for _, cli := range preference {
		take(cli)
	}
	return order
}
