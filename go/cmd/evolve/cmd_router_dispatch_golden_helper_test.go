package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func routerOverEvolveDir(t *testing.T, evolveDir string, pol policy.Policy) *cliroute.Router {
	t.Helper()
	dir := filepath.Join(evolveDir, "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r, _, err := cliroute.Build(cliroute.Setup{Policy: pol, Profiles: profiles.NewFromDir(dir),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func advisorRouter(t *testing.T, evolveDir string, rc policy.RouterPolicy) *cliroute.Router {
	t.Helper()
	return routerOverEvolveDir(t, evolveDir, policy.Policy{Router: &rc})
}

func mustRouterDispatch(t *testing.T, evolveDir string, rc policy.RouterPolicy) (string, string) {
	t.Helper()
	cli, model, err := resolveRouterDispatch(advisorRouter(t, evolveDir, rc), "")
	if err != nil {
		t.Fatalf("resolveRouterDispatch: %v", err)
	}
	return cli, model
}

func legacyAdvisorDispatch(t *testing.T, evolveDir, root string, pol policy.Policy, dt routerDecisionType, benched map[string]bool) (string, string, bool) {
	t.Helper()
	cli, model, ok, err := resolveRouterDispatchHealthy(routerOverEvolveDir(t, evolveDir, pol), root, dt, benched)
	if err != nil {
		t.Fatalf("resolveRouterDispatchHealthy: %v", err)
	}
	return cli, model, ok
}

func TestRouterDispatch_ComesFromTheTable(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(evolveDir, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "profiles", "router.json"), []byte(`{"name":"router","cli":"codex-tmux","model_tier_default":"deep"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := routerOverEvolveDir(t, evolveDir, policy.Policy{CLIRouting: &block})
	cli, model, ok, err := resolveRouterDispatchHealthy(r, "", decisionPlan, map[string]bool{})
	if err != nil || !ok || cli != "agy-tmux" || model != "deep" {
		t.Fatalf("the advisor routes on the table's primary at the router profile's tier: cli=%s model=%s ok=%v err=%v", cli, model, ok, err)
	}
	cli, _, ok, _ = resolveRouterDispatchHealthy(r, "", decisionPlan, map[string]bool{"agy": true})
	if !ok || cli != "claude-tmux" {
		t.Fatalf("the benched swap walks the table's chain: cli=%s ok=%v", cli, ok)
	}
	_, _, ok, _ = resolveRouterDispatchHealthy(r, "", decisionPlan, map[string]bool{"agy": true, "claude": true})
	if ok {
		t.Fatal("every candidate benched degrades the advisor")
	}
}

func TestRouterDispatch_ARefusedRouteIsAnError(t *testing.T) {
	evolveDir := t.TempDir()
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := routerOverEvolveDir(t, evolveDir, policy.Policy{CLIRouting: &block})
	t.Setenv("EVOLVE_ROUTER_CLI", "codex-tmux")
	if _, _, _, err := resolveRouterDispatchHealthy(r, "", decisionPlan, nil); err == nil {
		t.Fatal("an env primary outside the table refuses the advisor's route")
	}
	if _, _, err := resolveRouterDispatchFor(r, "", decisionPlan); err == nil {
		t.Fatal("resolveRouterDispatchFor surfaces the refusal")
	}
}
