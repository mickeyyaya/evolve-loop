package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

// runPreWaveProbes is the one pre-wave probe protocol of the loop and the
// campaign runner. It returns the context's error so a caller stops before
// dispatching a wave that would be cancelled at spawn.
func runPreWaveProbes(ctx context.Context, projectRoot, evolveDir string, env map[string]string, stderr io.Writer) error {
	runCLIHealthCanary(ctx, projectRoot, env, defaultLiveProbe(ctx, projectRoot, stderr), stderr)
	runUsageProbe(ctx, projectRoot, evolveDir, env, stderr)
	return ctx.Err()
}

// runUsageProbe benches every installed family already at a quota cap before
// the first phase boots. It is advisory and fails open.
func runUsageProbe(ctx context.Context, projectRoot, evolveDir string, env map[string]string, stderr io.Writer) {
	if !usageProbeEnabled(env, evolveDir) {
		return
	}
	families := bridge.InteractiveFamilies()
	if len(families) == 0 {
		return
	}
	fmt.Fprintf(stderr, "[loop] usage-probe: checking %v for quota caps before dispatch\n", families)
	newUsageProber(usageProbeDirs{projectRoot: projectRoot, evolveDir: evolveDir}, families, stderr).Run(ctx)
}

type usageProbeDirs struct {
	projectRoot, evolveDir string
}

func newUsageProber(dirs usageProbeDirs, families []string, stderr io.Writer) *usageprobe.Prober {
	factory := bridge.NewControllerFactory(dirs.projectRoot, filepath.Join(dirs.evolveDir, "usage-probe"), "usage-probe", bridge.Deps{})
	return &usageprobe.Prober{
		Families: families,
		Probe:    bridgeUsageProbe(factory),
		Classify: bridge.ClassifyExhausted,
		Windows:  usageevidence.ReadWindowsNow,
		Record:   recordUsageWindows(dirs.evolveDir),
		Store:    clihealth.NewStore(dirs.projectRoot, nil),
		Log:      stderr,
	}
}

func recordUsageWindows(evolveDir string) func(cli string, windows []quotastate.UsageWindow) error {
	return func(cli string, windows []quotastate.UsageWindow) error {
		return usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: cli, ObservedAt: time.Now().UTC(), Windows: windows})
	}
}

// bridgeUsageProbe is the single way to send a family's usage command over the
// bridge and read its pane, shared by the cap probe and the quota probe.
func bridgeUsageProbe(factory *bridge.ControllerFactory) func(ctx context.Context, family string) (string, error) {
	return usageevidence.UsagePane(factory)
}

func usageProbeEnabled(env map[string]string, evolveDir string) bool {
	if !envchain.BoolValue(envchain.Resolve("EVOLVE_CLI_HEALTH", env, "", "1"), true) {
		return false
	}
	return loadCLIHealthConfig(evolveDir).ProactiveProbe
}

// loadCLIHealthConfig returns the zero config, probe off, for an absent or
// malformed policy.
func loadCLIHealthConfig(evolveDir string) policy.CLIHealthConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.CLIHealthConfig{}
	}
	return pol.CLIHealthConfig()
}
