package carryover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestMergeAcceptanceDeferrals_ACriterionTriageDeferredCarriesToTheNextCycle(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	report := "## top_n\n- prune-guard: guard prune\n\n## deferred_acceptance\n- prune-guard: acceptance 2 — defer_reason=needs the continuation registry\n"
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	state := &cyclestate.State{}

	New().MergeAcceptanceDeferrals(state, ws, 1773, time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC))

	if len(state.CarryoverTodos) != 1 {
		t.Fatalf("carryover = %+v, want one todo for the deferred criterion", state.CarryoverTodos)
	}
	todo := state.CarryoverTodos[0]
	if todo.ID != "prune-guard-acceptance-2" || todo.FirstSeenCycle != 1773 ||
		!strings.Contains(todo.Action, "acceptance 2 of prune-guard") || !strings.Contains(todo.Action, "needs the continuation registry") {
		t.Errorf("todo = %+v; want a split carryover item that names the parent item, the criterion and the reason", todo)
	}
}

func TestMergeAcceptanceDeferrals_NoStateOrNoReportMintsNothing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	New().MergeAcceptanceDeferrals(nil, t.TempDir(), 1773, now)
	state := &cyclestate.State{}
	New().MergeAcceptanceDeferrals(state, "", 1773, now)
	New().MergeAcceptanceDeferrals(state, t.TempDir(), 1773, now)
	if len(state.CarryoverTodos) != 0 {
		t.Errorf("carryover = %+v, want none without a state, a workspace or a triage report", state.CarryoverTodos)
	}
}
