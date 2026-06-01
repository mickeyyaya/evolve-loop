package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

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

// classifierCLIPreference orders the CLIs we'd rather run the one-shot tier
// classification on (codex's `exec` is the most validated headless path).
var classifierCLIPreference = []string{"codex", "claude", "agy"}

// liveRefresh queries each ready CLI's live /model list, classifies the ids
// into tiers via a ready CLI, and assembles the catalog. Per-CLI live failures
// fall back to that CLI's detect tier map (modelquery.Refresh handles this), so
// the refresh is best-effort and never aborts.
func liveRefresh(ctx context.Context, rep setup.DetectReport, workspace string, log io.Writer) (modelcatalog.Catalog, error) {
	if workspace == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return modelcatalog.Catalog{}, fmt.Errorf("liveRefresh: resolve workspace: %w", err)
		}
		workspace = cwd
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
	classifierCLI := pickClassifierCLI(readyCLIs)

	capturer := bridgeModelCapturer{workspace: workspace}
	router := modelquery.Router{
		ByCLI:   map[string]modelquery.Lister{"ollama": modelquery.OllamaLister{}},
		Default: modelquery.RecipeLister{Capturer: capturer},
	}
	return modelquery.Refresh(ctx, modelquery.RefreshDeps{
		CLIs:       readyCLIs,
		Lister:     router,
		Classifier: modelquery.CLIClassifier{CLI: classifierCLI},
		Fallback:   fallback,
		Now:        time.Now,
		Log:        log,
	})
}

// pickClassifierCLI chooses which ready CLI runs the tier-classification prompt.
// EVOLVE_MODELCATALOG_CLASSIFIER_CLI overrides — but ONLY when it names a ready
// CLI (a stale/misconfigured override must not silently classify against a
// blocked CLI; mirrors the policy-pin validation discipline). Otherwise the
// first ready CLI in preference order, else any ready CLI.
func pickClassifierCLI(ready []string) string {
	readySet := make(map[string]bool, len(ready))
	for _, r := range ready {
		readySet[r] = true
	}
	if env := os.Getenv("EVOLVE_MODELCATALOG_CLASSIFIER_CLI"); env != "" && readySet[env] {
		return env
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
