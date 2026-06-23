package main

// cli_usage_probe.go wires the proactive per-cycle usage probe into the loop and
// campaign runners. It is the production assembly: enumerate installed
// interactive families, build a per-family-isolated bridge Controller, and run
// the usageprobe.Prober (which benches capped families into the shared clihealth
// store so the dispatcher's existing pre-skip demotes them). All of it is gated
// off by default — opt-in via policy.json cli_health.proactive_probe, with
// EVOLVE_CLI_HEALTH=0 as the master kill switch.

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge"
	"github.com/mickeyyaya/evolveloop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolveloop/go/internal/clihealth"
	"github.com/mickeyyaya/evolveloop/go/internal/envchain"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
	"github.com/mickeyyaya/evolveloop/go/internal/usageprobe"
)

// runUsageProbe probes every installed interactive family for a current quota
// cap and benches the capped ones BEFORE the cycle's first phase boots. No-op
// when disabled. Fail-open throughout — advisory, never blocks a cycle.
func runUsageProbe(projectRoot, evolveDir string, env map[string]string, stderr io.Writer) {
	if !usageProbeEnabled(env, evolveDir) {
		return
	}
	families := bridge.InteractiveFamilies()
	if len(families) == 0 {
		return
	}
	// The factory owns per-family bridge.Config assembly + workspace isolation,
	// so this wiring stays agnostic of how a probe session is built.
	factory := bridge.NewControllerFactory(projectRoot, filepath.Join(evolveDir, "usage-probe"), "usage-probe", bridge.Deps{})
	p := &usageprobe.Prober{
		Families: families,
		Probe: func(ctx context.Context, family string) (string, error) {
			resp, err := factory.For(family).Do(ctx, family, clicontrol.EventUsage)
			return resp.Pane, err
		},
		Classify: bridge.ClassifyExhausted,
		Store:    clihealth.NewStore(projectRoot, nil),
		Log:      stderr,
	}
	fmt.Fprintf(stderr, "[loop] usage-probe: checking %v for quota caps before dispatch\n", families)
	p.Run(context.Background())
}

// usageProbeEnabled reports whether the proactive probe should run: the
// EVOLVE_CLI_HEALTH master switch must not be 0 AND policy.json must opt in.
func usageProbeEnabled(env map[string]string, evolveDir string) bool {
	if !envchain.BoolValue(envchain.Resolve("EVOLVE_CLI_HEALTH", env, "", "1"), true) {
		return false
	}
	return loadCLIHealthConfig(evolveDir).ProactiveProbe
}

// loadCLIHealthConfig loads .evolve/policy.json and returns the CLI-health
// config. Absent or malformed policy ⇒ zero value (probe off).
func loadCLIHealthConfig(evolveDir string) policy.CLIHealthConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.CLIHealthConfig{}
	}
	return pol.CLIHealthConfig()
}
