package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PolicyPin_HonoredAbsolutely(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"),
		`{"pins":{"scout":{"cli":"codex-tmux","model":"claude-opus-4-8"}}}`)

	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		// An explicit per-agent env CLI, which must still lose to the pin.
		Env: map[string]string{"EVOLVE_SCOUT_CLI": "agy-tmux"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.CLI != "codex-tmux" {
		t.Errorf("CLI=%q, want codex-tmux (policy pin beats EVOLVE_SCOUT_CLI)", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "claude-opus-4-8" {
		t.Errorf("Model=%q, want claude-opus-4-8 (policy pin)", fb.gotReq.Model)
	}
}

func TestRun_PolicyPin_Bypass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"),
		`{"pins":{"scout":{"cli":"codex-tmux"}}}`)

	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		BypassPolicy: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.CLI != "claude-tmux" {
		t.Errorf("CLI=%q, want claude-tmux (default; pin bypassed)", fb.gotReq.CLI)
	}
}

func TestRun_PolicyPin_InvalidFailsLoudly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "profiles", "scout.json"),
		`{"name":"scout","cli":"claude-tmux","allowed_clis":["claude","agy"]}`)
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"),
		`{"pins":{"scout":{"cli":"codex-tmux"}}}`) // codex not in allowed_clis

	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected an error for an out-of-guardrail policy pin")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
	if fb.gotReq.Agent != "" {
		t.Error("bridge must NOT be launched when the policy pin is invalid")
	}
}

// Discriminating only on a host without the pinned binary but with a fallback's; elsewhere it passes trivially.
func TestRun_PolicyPin_NotDemotedByProbe(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "profiles", "scout.json"),
		`{"name":"scout","cli":"agy-tmux","cli_fallback":["claude-tmux"]}`)
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"),
		`{"pins":{"scout":{"cli":"codex-tmux"}}}`)

	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.CLI != "codex-tmux" {
		t.Errorf("CLI=%q, want codex-tmux (pinned primary must not be probe-demoted)", fb.gotReq.CLI)
	}
}

func TestRun_PolicyPin_MalformedFailsLoudly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"), `{ not json`)

	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("malformed policy.json must fail the phase loudly")
	}
}
