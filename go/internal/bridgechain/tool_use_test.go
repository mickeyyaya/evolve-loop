package bridgechain_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const cycleWorktree = "/wt/cycle-1"

func toolUseResolver(t *testing.T, cli string, fallback ...string) (bridgechain.PlanResolver, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	writeProfile(t, dir, "builder", map[string]any{"name": "builder", "cli": cli, "cli_fallback": fallback, "model_tier_default": "balanced"})
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{}, Profiles: profiles.NewFromDir(dir),
		Host: cliroute.Host{LookPath: findAll, Toolless: bridge.LacksToolUse},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return bridgechain.DefaultPlanResolver(r), root
}

func TestDefaultPlanResolver_AWorktreeLaunchDropsACLIWithoutToolUseWhoeverNamedIt(t *testing.T) {
	cases := []struct {
		name     string
		cli      string
		fallback []string
		mutate   func(*core.BridgeRequest)
		want     []string
	}{
		{"the profile cli", "ollama-tmux", []string{"claude-tmux"}, func(*core.BridgeRequest) {}, []string{"claude-tmux"}},
		{"the profile cli_fallback", "claude-tmux", []string{"ollama-tmux", "agy-tmux"}, func(*core.BridgeRequest) {}, []string{"claude-tmux", "agy-tmux"}},
		{"an EVOLVE_CLI override", "claude-tmux", []string{"agy-tmux"},
			func(r *core.BridgeRequest) { r.Env = map[string]string{"EVOLVE_CLI": "ollama-tmux"} }, []string{"agy-tmux"}},
		{"the caller cli", "claude-tmux", []string{"agy-tmux"}, func(r *core.BridgeRequest) { r.CLI = "ollama-tmux" }, []string{"claude-tmux", "agy-tmux"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolve, root := toolUseResolver(t, tc.cli, tc.fallback...)
			req := core.BridgeRequest{Agent: "builder", Model: "balanced", ProjectRoot: root, Worktree: cycleWorktree}
			tc.mutate(&req)

			plan := mustPlan(t, resolve, req)

			if !reflect.DeepEqual(plan.Candidates, tc.want) {
				t.Fatalf("chain = %v, want %v", plan.Candidates, tc.want)
			}
		})
	}
}

func TestDefaultPlanResolver_ALaunchWithNoWorktreeKeepsACLIWithoutToolUse(t *testing.T) {
	resolve, root := toolUseResolver(t, "ollama-tmux", "claude-tmux")

	plan := mustPlan(t, resolve, core.BridgeRequest{Agent: "builder", Model: "balanced", ProjectRoot: root})

	if want := []string{"ollama-tmux", "claude-tmux"}; !reflect.DeepEqual(plan.Candidates, want) {
		t.Fatalf("a launch that runs no tools keeps its chain: %v, want %v", plan.Candidates, want)
	}
}

func TestWalking_AWorktreeLaunchNeverReachesACLIWithoutToolUse(t *testing.T) {
	resolve, root := toolUseResolver(t, "ollama-tmux", "claude-tmux")
	inner := &scripted{}

	_, err := bridgechain.New(inner, resolve).Launch(context.Background(), core.BridgeRequest{Agent: "builder", Model: "balanced", ProjectRoot: root, Worktree: cycleWorktree})

	if err != nil || len(inner.calls) != 1 || inner.calls[0].CLI != "claude-tmux" {
		t.Fatalf("the walk launches only the tool-capable CLI: err=%v calls=%v", err, attempts(inner.calls))
	}
}

func TestWalking_AWorktreeLaunchWithNoToolCapableCLIFailsLoudlyAndLaunchesNothing(t *testing.T) {
	resolve, root := toolUseResolver(t, "ollama-tmux")
	inner := &scripted{}

	_, err := bridgechain.New(inner, resolve).Launch(context.Background(), core.BridgeRequest{Agent: "builder", Model: "balanced", ProjectRoot: root, Worktree: cycleWorktree})

	if err == nil || !strings.Contains(err.Error(), "ollama-tmux") || !strings.Contains(err.Error(), "builder") || len(inner.calls) != 0 {
		t.Fatalf("a chain with no tool-capable CLI fails the launch naming the dropped CLI: calls=%d err=%v", len(inner.calls), err)
	}
}
