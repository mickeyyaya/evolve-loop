package runner

import (
	"flag"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

var updateLegacyPlans = flag.Bool("update", false, "rewrite the legacy dispatch-plan golden")

func TestLegacyDispatchPlans_MatchTheGolden(t *testing.T) {
	resetUniversalFallbackDefaults(t)
	t.Setenv("EVOLVE_CLI", "")
	t.Setenv("EVOLVE_CLI_HEALTH", "")
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	if len(agents) < 50 {
		t.Fatalf("only %d tracked profiles — the golden lost its corpus", len(agents))
	}
	var runnerRecords, chainRecords []cliroutetest.Record
	for _, v := range cliroutetest.Variants() {
		root := v.ProjectRoot(t, cliroutetest.PhasesOf(agents))
		pol := v.Policy(t, root)
		t.Setenv("PATH", v.PathDir(t))
		for _, agent := range agents {
			runnerRecords = append(runnerRecords, runnerLegacyRecord(t, v, pol, root, profileDir, agent))
			chainRecords = append(chainRecords, bridgechainLegacyRecord(v, pol, root, profileDir, agent))
		}
	}
	cliroutetest.AssertGolden(t, append(runnerRecords, chainRecords...), *updateLegacyPlans)
}

func runnerLegacyRecord(t *testing.T, v cliroutetest.Variant, pol policy.Policy, root, profileDir, agent string) cliroutetest.Record {
	t.Helper()
	phase := cliroutetest.PhaseOf(agent)
	prof, err := profiles.NewFromDir(profileDir).Get(agent)
	if err != nil {
		t.Fatalf("profile %s: %v", agent, err)
	}
	b := New(Options{
		Hooks: &fakeHooks{phase: phase, agent: "evolve-" + agent, model: cliroutetest.DefaultModel},
		NowFn: cliroutetest.Now,
		ResolveLLM: func(string, resolvellm.Options) (resolvellm.Result, error) {
			return resolvellm.Result{}, cliroutetest.ErrDeclineAuto
		},
		UniversalFallback: pol.WorkflowConfig().UniversalFallback,
		DiscoverCLIsFn:    v.ProductionDiscover(pol),
		Diag:              log.Console{Out: io.Discard, Err: io.Discard},
	})
	prep := phasePreparation{
		phase: phase, profileDir: profileDir, profileName: agent,
		profilePath: filepath.Join(profileDir, agent+".json"), profile: &prof,
	}
	req := core.PhaseRequest{
		ProjectRoot: root, Env: v.EnvFor(agent), BypassPolicy: v.Bypass,
		ModelRoutingCLI: v.Overlay.CLI, ModelRoutingTier: v.Overlay.Tier,
	}
	resolved, resp, err := b.resolveDispatchPlan(req, prep)
	if err != nil {
		if resp == nil || len(resp.Diagnostics) == 0 {
			t.Fatalf("%s/%s: error without a diagnostic: %v", v.Name, agent, err)
		}
		return cliroutetest.ErrorRecord(cliroutetest.ResolverRunner, v.Name, agent, phase, resp.Diagnostics[0].Message)
	}
	return cliroutetest.PlanRecord(cliroutetest.ResolverRunner, v.Name, agent, phase, resolved.plan)
}

func bridgechainLegacyRecord(v cliroutetest.Variant, pol policy.Policy, root, profileDir, agent string) cliroutetest.Record {
	discover := v.ProductionDiscover(pol)
	resolve := bridgechain.DefaultPlanResolver(profileDir, func() []string {
		if discover == nil {
			return nil
		}
		return discover()
	}, v.LookPath, cliroutetest.Now, nil)
	plan := resolve(core.BridgeRequest{
		Agent: agent, Model: cliroutetest.DefaultModel, CLI: v.CallerCLI,
		Env: v.EnvFor(agent), ProjectRoot: root,
	})
	return cliroutetest.PlanRecord(cliroutetest.ResolverBridgechain, v.Name, agent, "", plan)
}
