package runner

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestResolveRouting_AdvisorOverlayPreservesFamilyTransport drives the advisor projection through
// BaseRunner.Run -> resolveDispatchPlan (routing.go); the contract-escalation projection, which also sets
// ModelRoutingCLI, is TestRunner_ContractEscalation_RedispatchesOnEscalatedCLIWithDirective.
func TestResolveRouting_AdvisorOverlayPreservesFamilyTransport(t *testing.T) {
	tests := []struct {
		name, primary, overlay, failCLI string
		fallback, want                  []string
	}{
		{name: "bare family keeps headless transport", primary: "claude-p", fallback: []string{"codex"}, overlay: "claude", want: []string{"claude-p"}},
		{name: "explicit driver wins and keeps fallback", primary: "claude-p", overlay: "claude-tmux", failCLI: "claude-tmux", want: []string{"claude-tmux", "claude-p"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeFallbackProfile(t, "evolve-scout", tt.primary, tt.fallback)
			hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
			bridge := &scriptedBridge{responses: map[string]scriptedResp{}}
			if tt.failCLI != "" {
				bridge.responses[tt.failCLI] = scriptedResp{
					resp: core.BridgeResponse{ExitCode: 80},
					err:  errors.New("bridge: launch exit=80"),
				}
			}
			runner := New(Options{Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-scout", "x")})

			if _, err := runner.Run(context.Background(), core.PhaseRequest{
				ProjectRoot:     root,
				Workspace:       t.TempDir(),
				ModelRoutingCLI: tt.overlay,
			}); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !slices.Equal(bridge.calls, tt.want) {
				t.Errorf("dispatch order=%v, want %v", bridge.calls, tt.want)
			}
		})
	}
}

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
