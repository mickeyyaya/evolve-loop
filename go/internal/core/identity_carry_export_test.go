package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func CarryAPendedIdenticalLaneForTesting(t *testing.T, persist func(string, CompositionVerdictInput) error) (projectRoot, worktree string, next Phase) {
	t.Helper()
	h := pendedIdenticalLane(t)
	h.persist = persist
	next, _, _ = h.route(t, h.auditRow())
	return h.fx.root, h.fx.worktree, next
}

type CompositionRungsForTesting struct {
	Worktree                    string
	TrivialRebase, ScopedReview func() bool
}

func RungsOnALinkedLedgerForTesting(t *testing.T, projectRoot string, persist func(string, CompositionVerdictInput) error) CompositionRungsForTesting {
	t.Helper()
	worktree, diff, patchID := divergedCompositionFixture(t)
	linkGuardDeps(worktree, projectRoot, unwindCycle)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionSnapshot(snapshotFromFixture(diff, patchID)),
		WithScopedMergeReviewer(func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome { return allComposedGatesPass() }),
		WithCompositionVerdictWriter(persist),
	)
	cs := CycleState{ActiveWorktree: worktree, RunID: "run"}
	return CompositionRungsForTesting{
		Worktree:      worktree,
		TrivialRebase: func() bool { return o.compositionCarryForward(context.Background(), unwindCycle, cs, projectRoot) },
		ScopedReview:  func() bool { return o.scopedMergeCarryForward(context.Background(), unwindCycle, cs, projectRoot) },
	}
}
