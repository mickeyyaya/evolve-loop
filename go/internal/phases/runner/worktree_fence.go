package runner

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

// takeWorktreeFence opens the fence for the dispatch (inert for a source
// writer or without a worktree) and logs an untakeable one at once.
// See ADR-0097.
func takeWorktreeFence(ctx context.Context, phase string, req core.PhaseRequest) *treefence.Fence {
	f := treefence.Begin(ctx, req.Worktree, req.WorktreeReadOnly)
	if err := f.TakeErr(); err != nil {
		log.Diag().Warnf("[runner] WARN worktree fence phase=%s: snapshot failed (%v) — the tree this phase hands downstream is unverified\n", phase, err)
	}
	return f
}

// restoreWorktreeFence closes the fence and renders what it did.
func restoreWorktreeFence(ctx context.Context, phase string, f *treefence.Fence) (bool, []core.Diagnostic) {
	outcome := f.End(ctx)
	diags := outcome.Diagnostics(phase)
	for _, d := range diags {
		log.Diag().Warnf("[runner] WARN %s\n", d.Message)
	}
	return outcome.Verified, diags
}
