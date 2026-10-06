package runner

import (
	"flag"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

var updateLegacyPlans = flag.Bool("update", false, "rewrite the legacy dispatch-plan golden")

func TestLegacyDispatchPlans_MatchTheGolden(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	t.Setenv("EVOLVE_CLI_HEALTH", "")
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	if len(agents) < 50 {
		t.Fatalf("only %d tracked profiles — the golden lost its corpus", len(agents))
	}
	var runnerRecords, chainRecords []cliroutetest.Record
	for _, v := range cliroutetest.Variants() {
		root := v.ProjectRoot(t, cliroutetest.PhasesOf(agents))
		router := goldenRouter(t, v, v.Policy(t, root), profileDir)
		t.Setenv("PATH", v.PathDir(t))
		for _, agent := range agents {
			runnerRecords = append(runnerRecords, runnerLegacyRecord(t, v, router, root, profileDir, agent))
			chainRecords = append(chainRecords, bridgechainLegacyRecord(v, router, root, agent))
		}
	}
	cliroutetest.AssertGolden(t, append(runnerRecords, chainRecords...), *updateLegacyPlans)
}

func goldenRouter(t *testing.T, v cliroutetest.Variant, pol policy.Policy, profileDir string) *cliroute.Router {
	t.Helper()
	router, _, err := cliroute.Build(cliroute.Setup{
		Policy: pol, Profiles: profiles.NewFromDir(profileDir),
		Host: cliroute.Host{Discover: v.RawDiscover, Bench: func(root, label string, plan llmroute.Plan, env map[string]string) llmroute.Plan {
			return bridgechain.ApplyCLIHealthBench(root, label, plan, env, cliroutetest.Now, nil)
		}},
	})
	if err != nil {
		t.Fatalf("variant %s: Build: %v", v.Name, err)
	}
	return router
}

func runnerLegacyRecord(t *testing.T, v cliroutetest.Variant, router *cliroute.Router, root, profileDir, agent string) cliroutetest.Record {
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
		Router: router,
		Diag:   log.Console{Out: io.Discard, Err: io.Discard},
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

func bridgechainLegacyRecord(v cliroutetest.Variant, router *cliroute.Router, root, agent string) cliroutetest.Record {
	plan, err := bridgechain.DefaultPlanResolver(router)(core.BridgeRequest{
		Agent: agent, Model: cliroutetest.DefaultModel, CLI: v.CallerCLI,
		Env: v.EnvFor(agent), ProjectRoot: root,
	})
	if err != nil {
		return cliroutetest.ErrorRecord(cliroutetest.ResolverBridgechain, v.Name, agent, "", err.Error())
	}
	return cliroutetest.PlanRecord(cliroutetest.ResolverBridgechain, v.Name, agent, "", plan)
}
