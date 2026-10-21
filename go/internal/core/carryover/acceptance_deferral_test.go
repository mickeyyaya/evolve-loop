package carryover

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"
)

func TestMergeAcceptanceDeferrals_ACriterionTriageDeferredCarriesToTheNextCycle(t *testing.T) {
	t.Parallel()
	state := &cyclestate.State{}
	deferrals := []triagedecision.AcceptanceDeferral{{ID: "prune-guard", Index: 2, Reason: "needs the continuation registry", Criterion: "a continuation-bound branch is never pruned"}}

	New().MergeAcceptanceDeferrals(state, deferrals, 1773, time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC))

	if len(state.CarryoverTodos) != 1 {
		t.Fatalf("carryover = %+v, want one todo for the deferred criterion", state.CarryoverTodos)
	}
	todo := state.CarryoverTodos[0]
	if todo.ID != "prune-guard-acceptance-2" || todo.FirstSeenCycle != 1773 || !strings.Contains(todo.Action, "acceptance 2 of prune-guard") ||
		!strings.Contains(todo.Action, "needs the continuation registry") || !strings.Contains(todo.Action, "a continuation-bound branch is never pruned") {
		t.Errorf("todo = %+v; want a split carryover item that names the parent item, the reason and the criterion's words", todo)
	}
}

func TestMergeAcceptanceDeferrals_NoStateOrNoDeferralMintsNothing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	New().MergeAcceptanceDeferrals(nil, []triagedecision.AcceptanceDeferral{{ID: "a", Index: 1}}, 1773, now)
	state := &cyclestate.State{}
	New().MergeAcceptanceDeferrals(state, nil, 1773, now)
	if len(state.CarryoverTodos) != 0 {
		t.Errorf("carryover = %+v, want none without a state or a deferral", state.CarryoverTodos)
	}
}
