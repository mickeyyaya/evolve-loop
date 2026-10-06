package main

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var tailDrivers = []string{"claude-tmux", "codex-tmux", "agy-tmux"}

func doctorResultsFor(v cliroutetest.Variant) []gobridge.DoctorResult {
	results := make([]gobridge.DoctorResult, 0, len(tailDrivers)+2)
	for _, driver := range v.Discovered {
		results = append(results, doctorResult(driver, true, "ready"))
	}
	for _, driver := range tailDrivers {
		if !slices.Contains(v.Discovered, driver) {
			results = append(results, doctorResult(driver, false, "blocked"))
		}
	}
	return append(results, doctorResult("ollama-tmux", true, "ready"), doctorResult("claude-p", true, "ready"))
}

func productionRouterFor(t *testing.T, v cliroutetest.Variant, root, profileDir string) *cliroute.Router {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(profileDir, routingProfilesDir(root)); err != nil {
		t.Fatal(err)
	}
	fakeRoutingDoctor(t, doctorResultsFor(v)...)
	router, _, err := loadCLIRouter(root, routingHost(io.Discard, cliroutetest.Now))
	if err != nil {
		t.Fatalf("variant %s: the production composition refuses: %v", v.Name, err)
	}
	return router
}

func TestProductionRouter_ReplaysTheLegacyGolden(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	t.Setenv("EVOLVE_CLI_HEALTH", "")
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	declineAuto := func(string) (string, bool) { return "", false }
	var runnerRecords, chainRecords []cliroutetest.Record
	for _, v := range cliroutetest.Variants() {
		root := v.ProjectRoot(t, cliroutetest.PhasesOf(agents))
		router := productionRouterFor(t, v, root, profileDir)
		t.Setenv("PATH", v.PathDir(t))
		for _, agent := range agents {
			phase := cliroutetest.PhaseOf(agent)
			d, err := router.Resolve(cliroute.Request{
				Agent: agent, Phase: phase, ProjectRoot: root, DefaultModel: cliroutetest.DefaultModel,
				Env: v.EnvFor(agent), Overlay: v.Overlay, Expand: declineAuto, BypassPolicy: v.Bypass,
			})
			runnerRecords = append(runnerRecords, goldenRecord(cliroutetest.ResolverRunner, v.Name, agent, phase, d, err))
			plan, err := bridgechain.DefaultPlanResolver(router)(core.BridgeRequest{
				Agent: agent, Model: cliroutetest.DefaultModel, CLI: v.CallerCLI, Env: v.EnvFor(agent), ProjectRoot: root,
			})
			chainRecords = append(chainRecords, goldenRecord(cliroutetest.ResolverBridgechain, v.Name, agent, "", cliroute.Decision{Plan: plan}, err))
		}
	}
	cliroutetest.AssertGolden(t, append(runnerRecords, chainRecords...), false)
}

func goldenRecord(resolver, variant, agent, phase string, d cliroute.Decision, err error) cliroutetest.Record {
	if err != nil {
		return cliroutetest.ErrorRecord(resolver, variant, agent, phase, err.Error())
	}
	return cliroutetest.PlanRecord(resolver, variant, agent, phase, d.Plan)
}
