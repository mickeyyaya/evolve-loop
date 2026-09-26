package runner

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// skillsCapturingBridge records each attempt's tier and skills and always exits 85, so the whole tier chain is walked.
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
