package runner

// The cycles-1231/1232/1234 batch-halt class (and the prior day's retro
// failures — the SAME defect at a non-fatal phase): pre-worktree phases
// (scout/intent/triage) and post-teardown retro dispatch with an EMPTY
// req.Worktree, and the CB.2 guard in fleet mode refuses the process-cwd
// fallback (correctly: cwd under a fleet supervisor may be another run's
// tree). Nothing made the working tree EXPLICIT for phases that legitimately
// have no per-cycle worktree — so every second-cycle scout in a fleet lane
// died exit=10 with identical fingerprints and the breaker halted the batch.
//
// The contract pinned here: the BRIDGE request's Worktree falls back to
// req.ProjectRoot — deliberate and explicit, satisfying CB.2's real goal
// (never an ACCIDENTAL directory) while preserving what pre-worktree phases
// have always run over (the cycle's own project root; in fleet lanes the
// supervisor's cwd equalled it, which is why single mode never saw this).
// PhaseRequest.Worktree stays EMPTY upstream: every consumer that keys on
// `Worktree == ""` to mean "no per-cycle worktree yet" (the build floor's
// skip, preservation, contract roots) keeps that meaning — only the dispatch
// working directory becomes explicit.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type worktreeRecordingBridge struct{ got core.BridgeRequest }

func (b *worktreeRecordingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.got = req
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("ok"), 0o644)
	}
	return core.BridgeResponse{Stdout: "ok"}, nil
}
func (b *worktreeRecordingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestRun_EmptyWorktreeDispatchesProjectRootExplicitly(t *testing.T) {
	t.Parallel()
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	br := &worktreeRecordingBridge{}
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS("evolve-scout", "body")})
	req := core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()} // Worktree deliberately empty
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.got.Worktree == "" {
		t.Fatalf("bridge request Worktree is EMPTY — under EVOLVE_FLEET=1 the tmux drivers refuse this (exit=10 'fleet mode: explicit worktree required'), which killed scouts 1231/1232/1234 and halted the batch; the dispatch must name the tree explicitly")
	}
	if br.got.Worktree != req.ProjectRoot {
		t.Fatalf("bridge request Worktree = %q, want req.ProjectRoot %q — the explicit fallback must be the cycle's own project root, nothing else", br.got.Worktree, req.ProjectRoot)
	}
}

func TestRun_RealWorktreeStillWinsOverProjectRoot(t *testing.T) {
	t.Parallel()
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	br := &worktreeRecordingBridge{}
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS("evolve-builder", "body")})
	req := core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: t.TempDir()}
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.got.Worktree != req.Worktree {
		t.Fatalf("bridge request Worktree = %q, want the cycle worktree %q — the fallback must never override a provisioned worktree", br.got.Worktree, req.Worktree)
	}
}
