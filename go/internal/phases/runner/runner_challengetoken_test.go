package runner

// runner_challengetoken_test.go — cycle-269 incident, prompt half: the
// bash→Go migration dropped the challenge-token prompt injection entirely
// (builder.json's challenge_token_required had NO Go consumer;
// resolved-prompt.txt carried zero mentions), so whether a builder echoed the
// token depended on it spontaneously reading scout-report line 2 — the claude
// fallback didn't, and a perfect build FAILed at audit. The runner now
// appends a deterministic token block (the TurnBudgetHint append precedent)
// for phases whose CONTRACT requires the echo, sourced from the minted
// <workspace>/challenge-token.txt.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// promptRecordingBridge captures the exact prompt the runner dispatches.
type promptRecordingBridge struct{ gotPrompt string }

func (b *promptRecordingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.gotPrompt = req.Prompt
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("ok"), 0o644)
	}
	return core.BridgeResponse{Stdout: "ok"}, nil
}
func (b *promptRecordingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func runWithToken(t *testing.T, phase, agent string, mintToken bool) string {
	t.Helper()
	ws := t.TempDir()
	if mintToken {
		if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte("67dffdcb2fb3ab46\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hooks := &fakeHooks{phase: phase, agent: agent, model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	br := &promptRecordingBridge{}
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS(agent, "body")})
	if _, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: ws}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return br.gotPrompt
}

func TestRun_TokenRequiredPhase_PromptCarriesTokenAndInstruction(t *testing.T) {
	t.Parallel()
	prompt := runWithToken(t, "build", "evolve-builder", true)
	if !strings.Contains(prompt, "67dffdcb2fb3ab46") {
		t.Fatal("the dispatched prompt must carry the minted token (cycle-269: resolved-prompt.txt had ZERO mentions — the agent was never told)")
	}
	if !strings.Contains(prompt, "verbatim") {
		t.Errorf("the block must instruct a verbatim echo into the report; prompt tail: %q", prompt[max(0, len(prompt)-300):])
	}
}

func TestRun_TokenRequiredPhase_NoTokenFile_PromptUnchanged(t *testing.T) {
	t.Parallel()
	prompt := runWithToken(t, "build", "evolve-builder", false)
	if strings.Contains(strings.ToLower(prompt), "challenge") {
		t.Errorf("no minted token ⇒ no block (byte-identical legacy prompt); got: %q", prompt)
	}
}

func TestRun_NonTokenPhase_PromptUnchangedEvenWithFile(t *testing.T) {
	t.Parallel()
	// scout MINTS the token; injecting an echo instruction into it would be
	// circular. Its prompt stays untouched even when the file exists.
	prompt := runWithToken(t, "scout", "evolve-scout", true)
	if strings.Contains(strings.ToLower(prompt), "challenge") {
		t.Errorf("non-token-required phase must not receive the block; got: %q", prompt)
	}
}
