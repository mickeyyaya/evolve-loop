package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func dispatchedEffort(t *testing.T, req core.PhaseRequest) string {
	t.Helper()
	hooks := &fakeHooks{phase: "build", agent: "evolve-build", model: "sonnet", prompt: "composed", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "# build report\n"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-build", "agent body"), VerifyFn: alwaysOKVerify})
	req.ProjectRoot, req.Workspace, req.Worktree = t.TempDir(), t.TempDir(), t.TempDir()
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return fb.gotReq.Effort
}

func TestRun_ALadderEffortReachesTheBridgeRequest(t *testing.T) {
	if got := dispatchedEffort(t, core.PhaseRequest{Assignment: core.TaskAssignment{Effort: "high"}}); got != "high" {
		t.Fatalf("bridge effort = %q, want the ladder effort high", got)
	}
	if got := dispatchedEffort(t, core.PhaseRequest{}); got != "" {
		t.Fatalf("bridge effort = %q without a ladder effort, want empty so the tier default holds", got)
	}
}
