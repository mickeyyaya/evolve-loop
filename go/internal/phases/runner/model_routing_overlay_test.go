package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestRunner_ModelRoutingAuto_SoftOverlayAppliesAsPrimary (mr4-projection
// AC1): with req.ModelRoutingCLI/Tier set (the cyclerun_dispatch seam's
// auto-mode projection, already clamped upstream), the runner's dispatch
// resolves the advisor's CLI as the chain PRIMARY and its tier as the
// dispatched model — NOT the profile's static default (profile.cli =
// claude-tmux, model_tier_default = sonnet; both would be dispatched absent
// the overlay).
func TestRunner_ModelRoutingAuto_SoftOverlayAppliesAsPrimary(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		ModelRoutingCLI:  "codex-tmux",
		ModelRoutingTier: "deep",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.CLI != "codex-tmux" {
		t.Errorf("CLI=%q, want codex-tmux (soft overlay promotes the advisor's CLI to chain primary)", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "deep" {
		t.Errorf("Model=%q, want deep (soft overlay tier, not the profile's model_tier_default=sonnet)", fb.gotReq.Model)
	}
}

// TestRunner_ModelRoutingAuto_ZeroOverlayByteIdentical (mr4-projection AC3
// counterpart, at the runner layer): a PhaseRequest with both overlay fields
// empty (static/advisory mode, or auto mode's no-proposal-for-this-phase
// case) dispatches EXACTLY the profile's static default — no soft overlay is
// constructed or applied.
func TestRunner_ModelRoutingAuto_ZeroOverlayByteIdentical(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.CLI != "claude-tmux" {
		t.Errorf("CLI=%q, want claude-tmux (profile default; no overlay proposed)", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "sonnet" {
		t.Errorf("Model=%q, want sonnet (profile.model_tier_default; no overlay proposed)", fb.gotReq.Model)
	}
}

// TestRunner_ModelRoutingAuto_BenchedOverlayPrimaryFallsBack (mr4-projection
// AC5, I3): the advisor proposes codex-tmux as the overlay CLI, but codex-tmux
// is ACTIVELY BENCHED (2-strike boot-timeout bench, same mechanism as
// cycle-426 driver bench). Because the overlay is SOFT (pin==nil), the
// dispatch chain must still fall back to the profile's original primary
// (claude-tmux) — a benched advisor choice must never collapse the chain to
// a single, unavailable candidate the way an absolute policy.Pin would.
func TestRunner_ModelRoutingAuto_BenchedOverlayPrimaryFallsBack(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", []string{"codex-tmux"})
	store := clihealth.NewStore(root, nil)
	for i := 0; i < clihealth.DefaultBootBenchThreshold; i++ {
		if _, err := store.RecordBootStrike("codex-tmux"); err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i+1, err)
		}
	}
	if _, ok := store.Active()["codex-tmux"]; !ok {
		t.Fatal("setup: codex-tmux not active after threshold strikes")
	}

	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: sb, Prompts: fakePromptsFS("evolve-auditor", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		ModelRoutingCLI: "codex-tmux", // the advisor's proposal — but it's benched
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sb.calls) == 0 || sb.calls[0] != "claude-tmux" {
		t.Errorf("dispatch order=%v, want claude-tmux first — a benched soft-overlay primary (pin==nil) must still fall back via the cli-health chain", sb.calls)
	}
}
