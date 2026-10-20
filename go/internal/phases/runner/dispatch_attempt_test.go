package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_TheBridgeRequestCarriesThePhaseDispatchAttempt(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-build", model: "sonnet", prompt: "composed", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "# build report\n"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-build", "agent body"), VerifyFn: alwaysOKVerify})

	if _, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: t.TempDir(), Attempt: 3}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.Attempt != 3 {
		t.Fatalf("bridge attempt = %d, want the phase request's 3", fb.gotReq.Attempt)
	}
}
