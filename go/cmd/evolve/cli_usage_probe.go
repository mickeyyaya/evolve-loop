package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func runPreWaveProbes(ctx context.Context, projectRoot, evolveDir string, stderr io.Writer) error {
	runCLIHealthCanary(ctx, projectRoot, defaultLiveProbe(ctx, projectRoot, bridge.Deps{Stderr: stderr}), stderr)
	runUsageProbe(ctx, projectRoot, evolveDir, stderr)
	recordLaneTimes(evolveDir, stderr)
	return ctx.Err()
}

func runUsageProbe(ctx context.Context, projectRoot, evolveDir string, stderr io.Writer) {
	if !usageProbeEnabled(evolveDir) {
		return
	}
	families := bridge.InteractiveFamilies()
	if len(families) == 0 {
		return
	}
	fmt.Fprintf(stderr, "[loop] usage-probe: checking %v for quota caps before dispatch\n", families)
	newUsageProber(usageProbeDirs{projectRoot: projectRoot, evolveDir: evolveDir}, families, stderr).Run(ctx)
}

var preWaveUsagePaneFn = bridgeUsageProbe

type usageProbeDirs struct {
	projectRoot, evolveDir string
}

func newUsageProber(dirs usageProbeDirs, families []string, stderr io.Writer) *usageprobe.Prober {
	factory := bridge.NewControllerFactory(dirs.projectRoot, filepath.Join(dirs.evolveDir, "usage-probe"), "usage-probe", bridge.Deps{})
	return &usageprobe.Prober{
		Families: families,
		Probe:    preWaveUsagePaneFn(factory),
		Classify: bridge.ClassifyExhausted,
		Windows:  usageevidence.ReadWindowsNow,
		Record:   recordUsageWindows(dirs.evolveDir),
		Store:    clihealth.NewStore(dirs.projectRoot, nil),
		Log:      stderr,
		Timeout:  loadCLIHealthConfig(dirs.evolveDir).UsageEvidenceTimeout(),
	}
}

func recordUsageWindows(evolveDir string) func(cli string, windows []quotastate.UsageWindow) error {
	return func(cli string, windows []quotastate.UsageWindow) error {
		return usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: cli, ObservedAt: time.Now().UTC(), Windows: windows})
	}
}

func bridgeUsageProbe(factory *bridge.ControllerFactory) func(ctx context.Context, family string) (string, error) {
	return usageevidence.UsagePane(factory)
}

func usageProbeEnabled(evolveDir string) bool {
	c := loadCLIHealthConfig(evolveDir)
	return c.On() && c.ProactiveProbe
}

func loadCLIHealthConfig(evolveDir string) policy.CLIHealthConfig {
	return policy.LoadOrDefaults(filepath.Join(evolveDir, "policy.json"), os.Stderr).CLIHealthConfig()
}
