package bridgechain_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func routerOver(t *testing.T, dir string, pol policy.Policy) *cliroute.Router {
	t.Helper()
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: pol, Profiles: profiles.NewFromDir(dir),
		Host: cliroute.Host{LookPath: findAll, Bench: func(root, label string, plan llmroute.Plan, env map[string]string) llmroute.Plan {
			return bridgechain.ApplyCLIHealthBench(root, label, plan, env, time.Now, nil)
		}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func legacyRouter(t *testing.T, dir string) *cliroute.Router {
	t.Helper()
	return routerOver(t, dir, policy.Policy{})
}

func mustPlan(t *testing.T, resolve bridgechain.PlanResolver, req core.BridgeRequest) llmroute.Plan {
	t.Helper()
	plan, err := resolve(req)
	if err != nil {
		t.Fatalf("resolve(%+v): %v", req, err)
	}
	return plan
}

func TestDefaultPlanResolver_DeclaredTableOutranksCallerCLI(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	writeProfile(t, dir, "retrospective", map[string]any{"name": "retrospective", "cli": "codex-tmux", "model_tier_default": "deep"})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	resolve := bridgechain.DefaultPlanResolver(routerOver(t, dir, policy.Policy{CLIRouting: &block}))
	plan := mustPlan(t, resolve, core.BridgeRequest{Agent: "retrospective", CLI: "codex-tmux", Model: "deep", ProjectRoot: root})
	if strings.Join(plan.Candidates, " ") != "agy-tmux claude-tmux" || plan.PrimarySource != "default" {
		t.Fatalf("a rule covering the agent outranks the caller's CLI: %+v", plan)
	}
}

func TestWalking_ARoutingRefusalFailsTheLaunchInsteadOfFallingBack(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	writeProfile(t, dir, "retrospective", map[string]any{"name": "retrospective", "cli": "codex-tmux"})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	inner := &scripted{}
	w := bridgechain.New(inner, bridgechain.DefaultPlanResolver(routerOver(t, dir, policy.Policy{CLIRouting: &block})))
	_, err := w.Launch(context.Background(), core.BridgeRequest{Agent: "retrospective", Model: "deep", ProjectRoot: root,
		Env: map[string]string{"EVOLVE_RETROSPECTIVE_CLI": "codex-tmux"}})
	if err == nil || !strings.Contains(err.Error(), "outside the allowed set") || len(inner.calls) != 0 {
		t.Fatalf("a bad rule fails the launch loudly and launches nothing: calls=%d err=%v", len(inner.calls), err)
	}
}

func TestWalking_AWalkThatLaunchesNothingSurfacesTheError(t *testing.T) {
	inner := &scripted{}
	w := bridgechain.New(inner, func(core.BridgeRequest) (llmroute.Plan, error) {
		return llmroute.Plan{Candidates: []string{"agy-tmux"}, Tiers: []string{"deep"}, Triggers: llmroute.DefaultTriggers(),
			PrimarySource: "agents:retrospective", TierCeiling: map[string][]string{"deep": {"claude"}}}, nil
	})
	_, err := w.Launch(context.Background(), core.BridgeRequest{Agent: "retrospective", CLI: "agy-tmux", Model: "deep"})
	if !errors.Is(err, llmroute.ErrNoPermittedAttempt) || len(inner.calls) != 0 || !strings.Contains(err.Error(), "agents:retrospective") {
		t.Fatalf("an empty walk is a loud failure naming the rule, never a zero-value success: calls=%d err=%v", len(inner.calls), err)
	}
}

func TestUnlaunched_OnlyAWalkWithNoAttemptAndNoWallIsRefused(t *testing.T) {
	plan := llmroute.Plan{PrimarySource: "default"}
	if err := bridgechain.Unlaunched(llmroute.TieredDispatchResult{Attempts: []string{"claude-tmux@deep"}}, plan); err != nil {
		t.Fatalf("a walk that launched is the keeper's to judge: %v", err)
	}
	if err := bridgechain.Unlaunched(llmroute.TieredDispatchResult{Walled: true}, plan); err != nil {
		t.Fatalf("a walled walk goes to the quota path: %v", err)
	}
	err := bridgechain.Unlaunched(llmroute.TieredDispatchResult{Err: llmroute.ErrNoPermittedAttempt}, plan)
	if !errors.Is(err, llmroute.ErrNoPermittedAttempt) || !errors.Is(err, bridgechain.ErrUnlaunched) {
		t.Fatalf("an empty unwalled walk is ErrUnlaunched and wraps walk.Err: %v", err)
	}
}

func TestUnlaunched_AnEmptyWalkWithNoErrorStillNamesTheCause(t *testing.T) {
	err := bridgechain.Unlaunched(llmroute.TieredDispatchResult{}, llmroute.Plan{PrimarySource: "default"})
	if !errors.Is(err, llmroute.ErrNoPermittedAttempt) || strings.Contains(err.Error(), "%!w") {
		t.Fatalf("a walk with no attempt and no error never formats a nil cause: %v", err)
	}
}
