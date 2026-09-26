package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
	factory := bridge.NewControllerFactory(projectRoot, filepath.Join(evolveDir, "usage-probe"), "usage-probe", bridge.Deps{})
	p := &usageprobe.Prober{
		Families: families,
		Probe:    bridgeUsageProbe(factory),
		Classify: bridge.ClassifyExhausted,
		Store:    clihealth.NewStore(projectRoot, nil),
		Log:      stderr,
	}
	fmt.Fprintf(stderr, "[loop] usage-probe: checking %v for quota caps before dispatch\n", families)
	p.Run(ctx)
}

// bridgeUsageProbe is the single way to send a family's usage command over the
// bridge and read its pane, shared by the cap probe and the quota probe.
func bridgeUsageProbe(factory *bridge.ControllerFactory) func(ctx context.Context, family string) (string, error) {
	return func(ctx context.Context, family string) (string, error) {
		resp, err := factory.For(family).Do(ctx, family, clicontrol.EventUsage)
		return resp.Pane, err
	}
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
