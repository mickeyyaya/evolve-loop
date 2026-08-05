package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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
	// Same seam as `evolve models refresh`. This path previously called
	// Write directly and silently destroyed operator-authored
	// tier_fallbacks on every cycle-start refresh.
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
// modelquery.PromptDispatcher (GAP 1, C1 fix): the tier-classification
// prompt is dispatched through the same sandboxed, liveness-probed,
// cli_fallback-aware bridge every phase uses, instead of a raw exec. It
// translates a base CLI name (codex|agy|claude) to the headless driver the
// bridge launches (driver_codex.go/driver_agy.go/driver_claudep.go), mirroring
// bridgeModelCapturer's cli->driver translation for the tmux pickers.
type bridgePromptDispatcher struct {
	workspace   string
	projectRoot string
}

func (d bridgePromptDispatcher) DispatchPrompt(ctx context.Context, cli, prompt string) (string, error) {
	driver := cli
	if cli == "claude" {
		driver = "claude-p"
	}
	eng := bridge.NewEngine(bridge.Deps{})
	resp, err := eng.Launch(ctx, core.BridgeRequest{
		CLI:          driver,
		Profile:      filepath.Join(d.projectRoot, ".evolve", "profiles", "router.json"),
		Prompt:       prompt,
		Workspace:    d.workspace,
		ProjectRoot:  d.projectRoot,
		Agent:        "model-classifier",
		ArtifactPath: filepath.Join(d.workspace, "model-classifier-artifact.txt"),
		Completion:   "artifact",
	})
	if err != nil {
		return "", fmt.Errorf("bridgePromptDispatcher: launch %s: %w", driver, err)
	}
	return resp.Stdout, nil
}

// classifierCLIPreference orders the CLIs we'd rather run the one-shot tier
// classification on (codex's `exec` is the most validated headless path).
var classifierCLIPreference = []string{"codex", "claude", "agy"}

// liveRefresh queries each ready CLI's live /model list, classifies the ids
// into tiers via a ready CLI (skipped wholesale when a CLI's offering
// fingerprints identically to prior — zero LLM calls on an unchanged
// offering), and assembles the catalog. Per-CLI live failures fall back to
// that CLI's detect tier map (modelquery.Refresh handles this), so the
// refresh is best-effort and never aborts.
func liveRefresh(ctx context.Context, rep setup.DetectReport, projectRoot, evolveDir string, prior modelcatalog.Catalog, log io.Writer) (modelcatalog.Catalog, error) {
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return modelcatalog.Catalog{}, fmt.Errorf("liveRefresh: resolve workspace: %w", err)
		}
		projectRoot = cwd
	}
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	var readyCLIs []string
	fallback := make(map[string]map[string]string)
	for _, c := range rep.CLIs {
		if c.Verdict != "ready" {
			continue
		}
		readyCLIs = append(readyCLIs, c.CLI)
		if len(c.TierModels) > 0 {
			fallback[c.CLI] = c.TierModels
		}
	}
	if len(readyCLIs) == 0 {
		return modelcatalog.Catalog{}, fmt.Errorf("no ready CLIs to query (run: evolve setup detect)")
	}
	// Non-empty by construction: pickClassifierCLI returns "" only for an empty
	// ready set, already excluded above.
	classifierCLI := pickClassifierCLI(readyCLIs, "")

	// The probe (tmux picker capture + one-shot classifier) works in a
	// throwaway scratch dir, never the repo: the router profile's sandbox
	// declares read_only_repo, so an ArtifactPath under the project root is
	// either denied (artifact-timeout per CLI at cycle start) or litters an
	// untracked file in main. The profile path stays anchored to projectRoot.
	// Diagnostics the bridge writes under the workspace (escalation reports,
	// launch errors, the llm-calls token ledger) are salvaged to a durable
	// home BEFORE teardown — deleting them with the scratch dir would silently
	// destroy the one artifact that explains a failed probe (Rule 12).
	scratch, err := os.MkdirTemp("", "evolve-models-probe-*")
	if err != nil {
		return modelcatalog.Catalog{}, fmt.Errorf("liveRefresh: scratch workspace: %w", err)
	}
	defer func() {
		salvageProbeDiagnostics(scratch, evolveDir, time.Now, log)
		_ = os.RemoveAll(scratch)
	}()

	capturer := bridgeModelCapturer{workspace: scratch}
	router := modelquery.Router{
		ByCLI:   map[string]modelquery.Lister{"ollama": modelquery.OllamaLister{}},
		Default: modelquery.RecipeLister{Capturer: capturer},
	}
	dispatcher := bridgePromptDispatcher{workspace: scratch, projectRoot: projectRoot}
	// D7 family gate: read policy.json catalog.allowed_families so each CLI's
	// live candidates are family-filtered before classification. Load is
	// nil-safe — an absent/malformed policy yields a nil map (no constraint),
	// byte-identical to today for every deployment that hasn't opted in.
	var allowedFamilies map[string][]string
	if pol, perr := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); perr == nil {
		allowedFamilies = pol.CatalogConfig().AllowedFamilies
	}
	return modelquery.Refresh(ctx, modelquery.RefreshDeps{
		CLIs:            readyCLIs,
		Lister:          router,
		Classifier:      modelquery.CLIClassifier{CLI: classifierCLI, Dispatcher: dispatcher},
		Fallback:        fallback,
		AllowedFamilies: allowedFamilies,
		Prior:           prior,
		Freshness:       freshnessFromManifests(readyCLIs),
		Now:             time.Now,
		Log:             log,
	})
}

// salvageProbeDiagnostics copies the diagnostic side-effects the bridge wrote
// under the scratch probe workspace into evolveDir/models-probe before the
// scratch dir is deleted (adversarial-review HIGH: a quota wall mid-probe
// writes escalation-report.json — pane tail + repair instructions — and the
// teardown used to delete the only copy while the refresh reported success):
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
func salvageProbeDiagnostics(scratch, evolveDir string, now func() time.Time, log io.Writer) {
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
			dst := fmt.Sprintf("escalation-report-%s.json", now().UTC().Format("20060102T150405Z"))
			if raw, rerr := os.ReadFile(src); rerr == nil && ensure() {
				if werr := os.WriteFile(filepath.Join(durable, dst), raw, 0o644); werr == nil {
					fmt.Fprintf(log, "[models] WARN probe escalation salvaged to %s — a live /model probe needed operator attention\n", filepath.Join(durable, dst))
				}
			}
		case strings.HasSuffix(name, "-launch-error.txt"):
			if raw, rerr := os.ReadFile(src); rerr == nil && ensure() {
				if werr := os.WriteFile(filepath.Join(durable, name), raw, 0o644); werr == nil {
					fmt.Fprintf(log, "[models] WARN probe launch-error salvaged to %s\n", filepath.Join(durable, name))
				}
			}
		case name == "llm-calls.ndjson":
			// O_APPEND append of a few-hundred-byte payload: relies on the
			// single-write(2) atomicity every other ndjson ledger writer in
			// this codebase already assumes (see sessionrecord.Append's
			// caveat) — fine at probe scale (≤ a handful of lines), torn
			// lines only become conceivable at multi-syscall payload sizes.
			if raw, rerr := os.ReadFile(src); rerr == nil && len(raw) > 0 && ensure() {
				if f, oerr := os.OpenFile(filepath.Join(durable, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); oerr == nil {
					_, _ = f.Write(raw)
					_ = f.Close()
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

// pickClassifierCLI chooses which ready CLI runs the tier-classification prompt.
// overrideCLI pins a specific CLI — but ONLY when it names a ready CLI (a
// stale/misconfigured override must not silently classify against a blocked CLI;
// mirrors the policy-pin validation discipline). Otherwise the first ready CLI
// in preference order, else any ready CLI.
func pickClassifierCLI(ready []string, overrideCLI string) string {
	readySet := make(map[string]bool, len(ready))
	for _, r := range ready {
		readySet[r] = true
	}
	if overrideCLI != "" && readySet[overrideCLI] {
		return overrideCLI
	}
	for _, pref := range classifierCLIPreference {
		if readySet[pref] {
			return pref
		}
	}
	if len(ready) > 0 {
		return ready[0]
	}
	return ""
}
