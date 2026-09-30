package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func dispatchedCompletion(t *testing.T, phase string, req core.PhaseRequest) core.CompletionContract {
	t.Helper()
	hooks := &fakeHooks{phase: phase, agent: "evolve-" + phase, model: "sonnet", prompt: "composed", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "# " + phase + " report\n"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-"+phase, "agent body"), VerifyFn: alwaysOKVerify})
	req.ProjectRoot, req.Workspace = t.TempDir(), t.TempDir()
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return fb.gotReq.Completion
}

func TestRun_DispatchesTheCompletionContractTheRequestNames(t *testing.T) {
	cases := []struct {
		name  string
		phase string
		req   core.PhaseRequest
		want  core.CompletionContract
	}{
		{"a source-writing correction completes on worktree evidence", "build",
			core.PhaseRequest{Worktree: t.TempDir(), CorrectionDirective: "fix the explanation doc"}, core.CompletionWorktreeEvidence},
		{"a first dispatch keeps the artifact contract", "build",
			core.PhaseRequest{Worktree: t.TempDir()}, ""},
		{"a read-only correction keeps the artifact contract", "audit",
			core.PhaseRequest{Worktree: t.TempDir(), CorrectionDirective: "fix the verdict", WorktreeReadOnly: true}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dispatchedCompletion(t, tc.phase, tc.req); got != tc.want {
				t.Fatalf("BridgeRequest.Completion = %q, want %q", got, tc.want)
			}
		})
	}
}
