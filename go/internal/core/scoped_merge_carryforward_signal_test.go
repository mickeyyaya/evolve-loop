package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestScopedMergeCarryForward_GateDeclineEmitsCodedSignalEvent(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithSignalCenter(center),
		WithCompositionSnapshot(snapshotFromFixture(diff, patchID)),
		WithScopedMergeReviewer(func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			return map[string]ciparity.GateOutcome{
				"compile":  {Status: "pass"},
				"test":     {Status: "pass"},
				"acs":      {Status: "fail", Tail: "TAIL_MARKER_SCOPED"},
				"apicover": {Status: "pass"},
			}
		}),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error {
			t.Fatal("writer must not be called after a composed-gate decline")
			return nil
		}),
	)

	if o.scopedMergeCarryForward(context.Background(), 7, CycleState{ActiveWorktree: worktree, RunID: "run-2"}, "") {
		t.Fatal("a red composed gate must not carry forward via the scoped-merge rung")
	}
	assertOneOrchestratorDeclineEvent(t, events, 7, "acs", "TAIL_MARKER_SCOPED")
}
