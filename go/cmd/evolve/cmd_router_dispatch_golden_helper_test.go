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
	rd, err := resolveRouterDispatchHealthy(advisorRouter(t, evolveDir, rc), "", decisionPlan, nil)
	if err != nil {
		t.Fatalf("resolveRouterDispatchHealthy: %v", err)
	}
	return rd.cli, rd.model
}

func legacyAdvisorDispatch(t *testing.T, evolveDir, root string, pol policy.Policy, dt routerDecisionType, benched map[string]bool) (string, string, bool) {
	t.Helper()
	rd, err := resolveRouterDispatchHealthy(routerOverEvolveDir(t, evolveDir, pol), root, dt, benched)
	if err != nil {
		t.Fatalf("resolveRouterDispatchHealthy: %v", err)
	}
	return rd.cli, rd.model, rd.healthy
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
	rd, err := resolveRouterDispatchHealthy(r, "", decisionPlan, map[string]bool{})
	if err != nil || !rd.healthy || rd.cli != "agy-tmux" || rd.model != "deep" {
		t.Fatalf("the advisor routes on the table's primary at the router profile's tier: %+v err=%v", rd, err)
	}
	rd, _ = resolveRouterDispatchHealthy(r, "", decisionPlan, map[string]bool{"agy": true})
	if !rd.healthy || rd.cli != "claude-tmux" {
		t.Fatalf("the benched swap walks the table's chain: %+v", rd)
	}
	rd, _ = resolveRouterDispatchHealthy(r, "", decisionPlan, map[string]bool{"agy": true, "claude": true})
	if rd.healthy {
		t.Fatal("every candidate benched degrades the advisor")
	}
}

func TestRouterDispatch_ARefusedRouteIsAnError(t *testing.T) {
	evolveDir := t.TempDir()
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := routerOverEvolveDir(t, evolveDir, policy.Policy{CLIRouting: &block})
	t.Setenv("EVOLVE_ROUTER_CLI", "codex-tmux")
	if _, err := resolveRouterDispatchHealthy(r, "", decisionPlan, nil); err == nil {
		t.Fatal("an env primary outside the table refuses the advisor's route")
	}
}
