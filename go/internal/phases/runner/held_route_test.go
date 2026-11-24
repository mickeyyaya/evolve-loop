package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func heldRouteBlock() policy.CLIRouting {
	return policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"claude", "agy"},
		Floors: policy.Floors{"claude": {"scout"}},
		Agents: map[string]policy.AgentRule{cliroute.HeldAuditKey: {CLI: []string{"claude", "agy"}, Model: "deep"}},
	}
}

func notClaude(cli, tier string) bool {
	return !strings.HasPrefix(cli, "claude") && policy.TierRank(tier) >= policy.TierRank("deep")
}

func TestRouteRequest_ARouteAgentReplacesTheProfileKeyAndClearsThePhase(t *testing.T) {
	t.Parallel()
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", nil)
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x")})

	plain := b.routeRequest(core.PhaseRequest{ProjectRoot: root}, scoutPreparation(root), nil)
	routed := b.routeRequest(core.PhaseRequest{ProjectRoot: root, RouteAgent: cliroute.HeldAuditKey}, scoutPreparation(root), nil)

	if plain.Agent != "scout" || plain.Phase != "scout" {
		t.Errorf("plain request = %s/%s, want the profile key and the phase", plain.Agent, plain.Phase)
	}
	if routed.Agent != cliroute.HeldAuditKey || routed.Phase != "" {
		t.Errorf("routed request = %s/%s, want the route key and no phase, so work rules never intercept the key", routed.Agent, routed.Phase)
	}
}

func TestResolveDispatchPlan_AnAdmitOnTheRequestReachesThePlanAndTheProfileFloorKeepsApplying(t *testing.T) {
	t.Parallel()
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", nil)
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x"),
		Router: declaredRouter(t, root, heldRouteBlock())})

	held, _, err := b.resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root, RouteAgent: cliroute.HeldAuditKey, Admit: notClaude}, scoutPreparation(root))
	if err != nil || held.plan.Admit == nil || held.plan.Admit("claude-tmux", "deep") {
		t.Fatalf("held plan %+v err %v, want the typed admission on the plan", held.plan, err)
	}
	if !strings.HasPrefix(held.plan.Candidates[0], "claude") || len(held.plan.Candidates) < 2 {
		t.Errorf("candidates = %v, want the policy chain with the builder family still listed first, for the admission to skip", held.plan.Candidates)
	}
	floored, _, err := b.resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root}, scoutPreparation(root))
	if err != nil || floored.plan.Admit != nil || len(floored.plan.Candidates) != 1 || !strings.HasPrefix(floored.plan.Candidates[0], "claude") {
		t.Errorf("floored plan %+v err %v, want the claude floor route with no admission, unchanged", floored.plan, err)
	}
}

func TestDispatchPhaseAttempts_AFirstCandidateOfTheBuilderFamilyIsSkippedAndTheWalkFallsThrough(t *testing.T) {
	t.Parallel()
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", nil)
	for name, admit := range map[string]func(string, string) bool{"with the admission": notClaude, "without it": nil} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fb := &fakeBridge{}
			b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x"),
				Router: declaredRouter(t, root, heldRouteBlock())})
			resolved, _, err := b.resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root, RouteAgent: cliroute.HeldAuditKey, Admit: admit}, scoutPreparation(root))
			if err != nil {
				t.Fatal(err)
			}

			b.dispatchPhaseAttempts(context.Background(), core.PhaseRequest{Workspace: t.TempDir()}, scoutPreparation(root), resolved)

			wantAgy := admit != nil
			if got := strings.HasPrefix(fb.gotReq.CLI, "agy"); got != wantAgy {
				t.Errorf("launched %q, want agy=%v: the admission skips the builder-family first candidate and walks on to the independent one", fb.gotReq.CLI, wantAgy)
			}
		})
	}
}
