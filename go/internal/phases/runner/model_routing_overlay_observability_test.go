package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
)

func writePolicyPin(t *testing.T, root, phase, cli, model string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `{"pins": {"` + phase + `": {"cli":"` + cli + `","model":"` + model + `"}}}`
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
}

func TestRunner_AdvisorOverlayAppliedLogsLine(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	var buf strings.Builder
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x"), Diag: log.Console{Out: &buf, Err: &buf}})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		ModelRoutingCLI:  "codex-tmux",
		ModelRoutingTier: "deep",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "[runner] phase=scout advisor overlay cli=codex-tmux tier=deep") {
		t.Errorf("diag log missing the overlay-applied line; got:\n%s", got)
	}
}

func TestRunner_NoAdvisorOverlayLogsProfileDefault(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	var buf strings.Builder
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x"), Diag: log.Console{Out: &buf, Err: &buf}})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "[runner] phase=scout no advisor overlay (profile default)") {
		t.Errorf("diag log missing the no-overlay line; got:\n%s", got)
	}
}

func TestRunner_PolicyPinWinsOverAdvisorOverlay_SourceIsPin(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	writePolicyPin(t, root, "scout", "claude-tmux", "opus")
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	var buf strings.Builder
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x"), Diag: log.Console{Out: &buf, Err: &buf}})

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		ModelRoutingCLI:  "codex-tmux",
		ModelRoutingTier: "deep",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.CLI != "claude-tmux" {
		t.Errorf("dispatched CLI=%q, want claude-tmux — a policy pin must win over the soft advisor overlay", fb.gotReq.CLI)
	}
	if resp.ModelSource != "pin" {
		t.Errorf("resp.ModelSource=%q, want %q (pin wins; must never be attributed to advisor)", resp.ModelSource, "pin")
	}
	if strings.Contains(buf.String(), "advisor overlay") {
		t.Errorf("diag log must not claim an advisor overlay applied when a pin won; got:\n%s", buf.String())
	}
}

func TestRun_ModelSourceReflectsResolutionPath(t *testing.T) {
	cases := []struct {
		name       string
		pinPhase   string
		overlayCLI string
		wantSource string
	}{
		{name: "profile default, no pin no overlay", wantSource: "profile"},
		{name: "advisor overlay, no pin", overlayCLI: "codex-tmux", wantSource: "advisor"},
		{name: "policy pin present", pinPhase: "scout", overlayCLI: "codex-tmux", wantSource: "pin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
			if tc.pinPhase != "" {
				writePolicyPin(t, root, tc.pinPhase, "claude-tmux", "opus")
			}
			hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
			fb := &fakeBridge{writeArtifact: "x"}
			r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

			resp, err := r.Run(context.Background(), core.PhaseRequest{
				ProjectRoot: root, Workspace: t.TempDir(),
				ModelRoutingCLI: tc.overlayCLI,
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if resp.ModelSource != tc.wantSource {
				t.Errorf("ModelSource=%q, want %q", resp.ModelSource, tc.wantSource)
			}
			if resp.ResolvedModel == "" {
				t.Errorf("ResolvedModel must never be empty when a phase dispatches")
			}
		})
	}
}
