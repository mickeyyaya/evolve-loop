package runner

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func declaredRouter(t *testing.T, root string, block policy.CLIRouting) *cliroute.Router {
	t.Helper()
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy:   policy.Policy{CLIRouting: &block},
		Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host:     cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func scoutPreparation(root string) phasePreparation {
	dir := filepath.Join(root, ".evolve", "profiles")
	prof, _ := profiles.NewFromDir(dir).Get("scout")
	return phasePreparation{phase: "scout", profileDir: dir, profileName: "scout", profilePath: filepath.Join(dir, "scout.json"), profile: &prof}
}

func TestResolveDispatchPlan_TheInjectedRouterDecidesThePlan(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", []string{"claude-tmux"})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x"),
		Router: declaredRouter(t, root, block)})
	resolved, resp, err := b.resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root}, scoutPreparation(root))
	if err != nil || resp != nil {
		t.Fatalf("resolveDispatchPlan: %v %+v", err, resp)
	}
	if got := resolved.plan.Candidates; len(got) != 2 || got[0] != "agy-tmux" || resolved.plan.PrimarySource != "default" {
		t.Fatalf("the table's chain routes the phase, not the profile's codex primary: %+v", resolved.plan)
	}
}

func TestResolveDispatchPlan_TheRootsDefaultRouterIsUsedWhenNoneIsInjected(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", nil)
	orig := DefaultRouter
	t.Cleanup(func() { DefaultRouter = orig })
	DefaultRouter = declaredRouter(t, root, policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}})
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x")})
	resolved, _, err := b.resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root}, scoutPreparation(root))
	if err != nil || resolved.plan.Candidates[0] != "claude-tmux" {
		t.Fatalf("the composition root's router routes every runner: %+v %v", resolved.plan, err)
	}
}

func TestResolveDispatchPlan_ARouterRefusalIsALoudFail(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", nil)
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x"),
		Router: declaredRouter(t, root, block)})
	req := core.PhaseRequest{ProjectRoot: root, Env: map[string]string{"EVOLVE_SCOUT_CLI": "codex-tmux"}}
	_, resp, err := b.resolveDispatchPlan(req, scoutPreparation(root))
	if err == nil || resp == nil || resp.Verdict != core.VerdictFAIL || !strings.Contains(resp.Diagnostics[0].Message, "outside the allowed set") {
		t.Fatalf("an env primary outside the table fails the phase loudly: %v %+v", err, resp)
	}
}

func TestDispatchPhaseAttempts_AWalkThatLaunchesNothingIsALoudSystemFail(t *testing.T) {
	fb := &fakeBridge{}
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})
	plan := phaseDispatchPlan{plan: llmroute.Plan{
		Candidates: []string{"agy-tmux"}, Tiers: []string{"deep"}, PrimarySource: "agents:scout",
		TierCeiling: map[string][]string{"deep": {"claude"}},
	}}
	got := b.dispatchPhaseAttempts(context.Background(), core.PhaseRequest{Workspace: t.TempDir()}, phasePreparation{phase: "scout"}, plan)
	if fb.gotReq.CLI != "" {
		t.Fatalf("the ceiling permits no attempt, so nothing launches: %+v", fb.gotReq)
	}
	if !errors.Is(got.bridgeErr, llmroute.ErrNoPermittedAttempt) || !strings.Contains(got.bridgeErr.Error(), "agents:scout") || !strings.Contains(got.bridgeErr.Error(), "map[deep:[claude]]") {
		t.Fatalf("an empty walk surfaces walk.Err with the rule and the ceiling, never a silent success: %v", got.bridgeErr)
	}
	if !errors.Is(got.bridgeErr, bridgechain.ErrUnlaunched) {
		t.Fatal("the result is marked unlaunched, so Run fails the phase as a system failure")
	}
}

func TestRun_AWalkThatLaunchesNothingFailsThePhaseWithAnError(t *testing.T) {
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x")})
	resp, err := b.unlaunchedFailure(core.PhaseRequest{Workspace: "/ws"}, phasePreparation{phase: "scout"}, phaseDispatchResult{bridgeErr: llmroute.ErrNoPermittedAttempt})
	if err == nil || resp.Verdict != core.VerdictFAIL || resp.Phase != "scout" || !strings.Contains(resp.Diagnostics[0].Message, "no attempt") {
		t.Fatalf("an unlaunched walk is a FAIL carrying the error: %+v %v", resp, err)
	}
}
