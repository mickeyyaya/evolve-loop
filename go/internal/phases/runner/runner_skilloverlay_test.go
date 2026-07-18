package runner

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// skillsCapturingBridge records the (tier, skills) of every dispatch attempt and
// always returns exit=85 so the runner walks the WHOLE tier-fallback chain. It
// lets a test assert the overlay skill set is recomputed PER ATTEMPT (differs
// between the resolved tier and the stepped-down tier), not resolved once.
type skillsCapturingBridge struct {
	attempts []skillsAttempt
}

type skillsAttempt struct {
	tier   string
	skills []string
}

func (b *skillsCapturingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.attempts = append(b.attempts, skillsAttempt{tier: req.Model, skills: req.Skills})
	return core.BridgeResponse{ExitCode: 85, Stderr: "quota exhausted"}, errors.New("bridge: launch exit=85")
}

func (b *skillsCapturingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// runner_skilloverlay_test.go — the PRODUCER half of the config-driven
// skill-overlay: the runner resolves policy overlays (which skill for which
// phase-agent dispatch) onto BridgeRequest.Skills so the adapter can preload
// them. These guard the "green resolver, inert dispatch" dormancy the feature
// was stuck in — they assert the resolved names actually reach the launch AND
// that the tier string at the dispatch site is the value the rule keys on.

// TestRunner_DeepTierDispatch_ResolvesFableOverlay: a phase dispatched at the
// deep tier carries the compiled-default overlay skill (fable) on
// BridgeRequest.Skills. Precondition Model=="deep" proves the dispatched tier
// string is literally "deep" (not a concrete model), the value the compiled
// {tiers:[deep,top]}→[fable] rule matches on.
func TestRunner_DeepTierDispatch_ResolvesFableOverlay(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-auditor", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		ModelRoutingTier: "deep",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.Model != "deep" {
		t.Fatalf("precondition: dispatched Model=%q, want deep (the tier string the overlay rule keys on)", fb.gotReq.Model)
	}
	if !reflect.DeepEqual(fb.gotReq.Skills, []string{"fable"}) {
		t.Errorf("gotReq.Skills=%v, want [fable] — the runner must resolve the deep-tier overlay onto BridgeRequest.Skills", fb.gotReq.Skills)
	}
}

// TestRunner_BalancedTierDispatch_NoOverlay: the negative — the profile-default
// (sonnet) dispatch carries no overlay skills, since the compiled default is
// deep/top only. Lower-tier dispatches stay byte-identical (Skills nil).
func TestRunner_BalancedTierDispatch_NoOverlay(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.Model != "sonnet" {
		t.Fatalf("precondition: dispatched Model=%q, want sonnet (profile default)", fb.gotReq.Model)
	}
	if len(fb.gotReq.Skills) != 0 {
		t.Errorf("gotReq.Skills=%v, want none — a non-deep/top tier has no compiled overlay", fb.gotReq.Skills)
	}
}

// TestRunner_TierStepDown_RecomputesOverlayPerAttempt locks the CENTRAL claim:
// overlay resolution lives INSIDE the tier-fallback closure, so a quota-wall
// step-down (deep→balanced) recomputes the skill set for the NEW tier. A profile
// with model_tier_default="deep" and the universal "balanced" floor walks the
// single-CLI chain twice — deep, then balanced — under an all-exit-85 bridge.
// The deep attempt must carry [fable]; the balanced attempt must carry none.
// A refactor that hoisted overlay resolution ABOVE the closure (resolving once
// for the first tier) would give both attempts the same skills and fail here.
func TestRunner_TierStepDown_RecomputesOverlayPerAttempt(t *testing.T) {
	root := writeQuotaExhaustionProfile(t, "evolve-auditor", "claude-tmux", "deep", nil)
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "deep", prompt: "x"}
	b := &skillsCapturingBridge{}
	r := New(Options{Hooks: hooks, Bridge: b, Prompts: fakePromptsFS("evolve-auditor", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("expected the exhausted chain to surface an error, got nil")
	}
	if len(b.attempts) != 2 {
		t.Fatalf("attempts=%+v, want 2 (deep then balanced — TierChain floor)", b.attempts)
	}
	if b.attempts[0].tier != "deep" || !reflect.DeepEqual(b.attempts[0].skills, []string{"fable"}) {
		t.Errorf("attempt[0]=%+v, want {tier:deep skills:[fable]}", b.attempts[0])
	}
	if b.attempts[1].tier != "balanced" || len(b.attempts[1].skills) != 0 {
		t.Errorf("attempt[1]=%+v, want {tier:balanced skills:[]} — overlay MUST be recomputed per attempt (hoisting it out of the closure would reuse the deep skills here)", b.attempts[1])
	}
}
