package retro

// retro_dispatch_cli_test.go — the cycle-107 class, pinned at the DISPATCH
// level this time: retro's hand-rolled runner consulted only EVOLVE_CLI and
// hardcoded claude-tmux, so editing retrospective.json's cli had no effect —
// the 2026-08-26 deep-tier sol arrangement's flagship flip (retro = ~40% of
// deep dispatch volume) was dead on arrival until review reproduced it
// against the dispatched BridgeRequest. These pins assert the REQUEST the
// bridge receives, never the JSON file.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeRetroProfile(t *testing.T, root, cli string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"name": "retrospective", "cli": cli, "model_tier_default": "deep"}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(dir, "retrospective.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_DispatchesProfileCLI(t *testing.T) {
	root := t.TempDir()
	writeRetroProfile(t, root, "codex-tmux")
	fb := &fakeBridge{resp: core.BridgeResponse{ExitCode: 0}, writeArtifact: "# retro\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Workspace: t.TempDir(), ProjectRoot: root, Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if fb.gotReq.CLI != "codex-tmux" {
		t.Fatalf("dispatched CLI = %q, want codex-tmux from retrospective.json — profile.cli must reach the BridgeRequest (cycle-107 class)", fb.gotReq.CLI)
	}
}

func TestRun_EnvCLIOverridesProfile(t *testing.T) {
	root := t.TempDir()
	writeRetroProfile(t, root, "codex-tmux")
	fb := &fakeBridge{resp: core.BridgeResponse{ExitCode: 0}, writeArtifact: "# retro\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Workspace: t.TempDir(), ProjectRoot: root, Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
		Env:     map[string]string{"EVOLVE_CLI": "claude-tmux"},
	})
	if fb.gotReq.CLI != "claude-tmux" {
		t.Fatalf("dispatched CLI = %q, want claude-tmux — the operator env override must stay first in the chain", fb.gotReq.CLI)
	}
}

func TestRun_NoProfileFallsBackToClaude(t *testing.T) {
	fb := &fakeBridge{resp: core.BridgeResponse{ExitCode: 0}, writeArtifact: "# retro\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Workspace: t.TempDir(), ProjectRoot: t.TempDir(), Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if fb.gotReq.CLI != "claude-tmux" {
		t.Fatalf("dispatched CLI = %q, want the claude-tmux default when no profile exists", fb.gotReq.CLI)
	}
}
