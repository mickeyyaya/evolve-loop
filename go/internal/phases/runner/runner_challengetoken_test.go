package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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
	// scout mints the token, so its prompt never carries the echo instruction.
	prompt := runWithToken(t, "scout", "evolve-scout", true)
	if strings.Contains(strings.ToLower(prompt), "challenge") {
		t.Errorf("non-token-required phase must not receive the block; got: %q", prompt)
	}
}
