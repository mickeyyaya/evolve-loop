package runner

// worktree_fence.go — the runner's use of the read-only phase worktree fence
// (ADR-0097). core marks a dispatch WorktreeReadOnly from its one
// write-permission predicate; the runner opens the fence before the first
// attempt and closes it after the last, so the tree every downstream reader
// sees — the audit's explanation binding inside Classify, the retro, the ship
// — is the tree the phase was given. What the fence did is a diagnostic on the
// phase response (report, dashboard, retro) and a WARN in the phase log; it is
// never silently kept and never silently dropped, and it never changes a
// verdict. The retro phase, which calls the bridge itself, holds the same
// treefence.Fence around its launch.
//
// Assumption, stated: the agent is gone when the fence closes. Launch is
// synchronous and the bridge tears the session down before returning; a
// driver that hands back a still-running REPL (resume-preserve) could write
// after the close — those writes reach the ship's own explanation
// verification, which stays armed.

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

// takeWorktreeFence opens the fence for the dispatch (inert for a source
// writer or without a worktree) and logs an untakeable one at once.
func takeWorktreeFence(ctx context.Context, phase string, req core.PhaseRequest) *treefence.Fence {
	f := treefence.Begin(ctx, req.Worktree, req.WorktreeReadOnly)
	if err := f.TakeErr(); err != nil {
		log.Diag().Warnf("[runner] WARN worktree fence phase=%s: snapshot failed (%v) — the tree this phase hands downstream is unverified\n", phase, err)
	}
	return f
}

// restoreWorktreeFence closes the fence and renders what it did.
func restoreWorktreeFence(ctx context.Context, phase string, f *treefence.Fence) []core.Diagnostic {
	diags := f.End(ctx).Diagnostics(phase)
	for _, d := range diags {
		log.Diag().Warnf("[runner] WARN %s\n", d.Message)
	}
	return diags
}
