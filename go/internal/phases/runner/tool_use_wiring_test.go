package runner

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func toolUseRouter(t *testing.T, root string, pol policy.Policy) *cliroute.Router {
	t.Helper()
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy:   pol,
		Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host:     cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }, Toolless: bridge.LacksToolUse},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func scoutRunner(router *cliroute.Router) *BaseRunner {
	return New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x"), Router: router})
}

const cycleWorktree = "/wt/cycle-1"

func TestResolveDispatchPlan_AWorktreePhaseDropsACLIWithoutToolUseWhoeverNamedIt(t *testing.T) {
	cases := []struct {
		name     string
		cli      string
		fallback []string
		pol      policy.Policy
		mutate   func(*core.PhaseRequest)
		want     []string
	}{
		{"the profile cli", "ollama-tmux", []string{"claude-tmux"}, policy.Policy{}, func(*core.PhaseRequest) {}, []string{"claude-tmux"}},
		{"the profile cli_fallback", "claude-tmux", []string{"ollama-tmux", "agy-tmux"}, policy.Policy{}, func(*core.PhaseRequest) {}, []string{"claude-tmux", "agy-tmux"}},
		{"an EVOLVE_CLI override", "claude-tmux", []string{"agy-tmux"}, policy.Policy{},
			func(r *core.PhaseRequest) { r.Env = map[string]string{"EVOLVE_CLI": "ollama-tmux"} }, []string{"agy-tmux"}},
		{"the advisor overlay", "claude-tmux", []string{"agy-tmux"}, policy.Policy{},
			func(r *core.PhaseRequest) { r.Driver = llmroute.Bind("ollama") }, []string{"claude-tmux", "agy-tmux"}},
		{"a policy pin", "claude-tmux", []string{"agy-tmux"}, policy.Policy{Pins: map[string]policy.Pin{"scout": {CLI: "ollama"}}},
			func(*core.PhaseRequest) {}, []string{"agy-tmux"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFallbackProfile(t, "evolve-scout", tc.cli, tc.fallback)
			req := core.PhaseRequest{ProjectRoot: root, Worktree: cycleWorktree}
			tc.mutate(&req)

			resolved, resp, err := scoutRunner(toolUseRouter(t, root, tc.pol)).resolveDispatchPlan(req, scoutPreparation(root))

			if err != nil || resp != nil {
				t.Fatalf("resolveDispatchPlan: %v %+v", err, resp)
			}
			if !reflect.DeepEqual(resolved.plan.Candidates, tc.want) {
				t.Fatalf("chain = %v, want %v", resolved.plan.Candidates, tc.want)
			}
		})
	}
}

func TestResolveDispatchPlan_APhaseWithNoWorktreeKeepsACLIWithoutToolUse(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "ollama-tmux", []string{"claude-tmux"})

	resolved, resp, err := scoutRunner(toolUseRouter(t, root, policy.Policy{})).resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root}, scoutPreparation(root))

	if want := []string{"ollama-tmux", "claude-tmux"}; err != nil || resp != nil || !reflect.DeepEqual(resolved.plan.Candidates, want) {
		t.Fatalf("a phase that runs no tools keeps its chain: %v %+v %v, want %v", resolved.plan.Candidates, resp, err, want)
	}
}

func TestResolveDispatchPlan_AWorktreeChainOfToollessCLIsIsALoudFailNamingThem(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "ollama-tmux", nil)

	_, resp, err := scoutRunner(toolUseRouter(t, root, policy.Policy{})).resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root, Worktree: cycleWorktree}, scoutPreparation(root))

	if err == nil || resp == nil || resp.Verdict != core.VerdictFAIL || !strings.Contains(resp.Diagnostics[0].Message, "ollama-tmux") {
		t.Fatalf("a chain with no tool-capable CLI fails the phase and names the dropped CLI: %v %+v", err, resp)
	}
}

func TestResolveDispatchPlan_TheLaunchRouterOfARunnerWithNoRootRouterFiltersToo(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "ollama-tmux", []string{"claude-tmux"})
	orig := DefaultRouter
	t.Cleanup(func() { DefaultRouter = orig })
	DefaultRouter = nil

	resolved, resp, err := scoutRunner(nil).resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root, Worktree: cycleWorktree}, scoutPreparation(root))

	if err != nil || resp != nil || !reflect.DeepEqual(resolved.plan.Candidates, []string{"claude-tmux"}) {
		t.Fatalf("the per-launch router judges tool use with the bridge manifests: %+v %+v %v", resolved.plan, resp, err)
	}
}
