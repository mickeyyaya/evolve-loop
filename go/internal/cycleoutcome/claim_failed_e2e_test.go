package cycleoutcome

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func TestFailureCloseout_ACycle1757ShapedLaneChargesItsPinnedItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeJSON(t, filepath.Join(root, ".evolve", "inbox", "pinned.json"), map[string]any{"id": "goal-text-has-no-selection-authority", "weight": 0.81})
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1757")
	writeJSON(t, filepath.Join(ws, "lane-scope.json"), map[string]any{"todo_ids": []string{"goal-text-has-no-selection-authority"}, "goal_hash": "h"})
	writeJSON(t, filepath.Join(ws, "triage-decision.json"), map[string]any{
		"top_n":          []any{},
		"escalate_block": []map[string]string{{"task_id": "goal-text-selection-authority", "reason": "protected-surface: go/internal/loopwave/loopwave.go"}},
	})
	writeJSON(t, filepath.Join(ws, phasetiming.FileName), []phasetiming.Entry{
		{Phase: "scout", Verdict: cyclestate.VerdictPASS, AttemptCount: 1},
		{Phase: "triage", Verdict: cyclestate.VerdictPASS, AttemptCount: 1, Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityWarning, Code: cyclestate.DiagCodeTriageProtectedSurface, Subject: "goal-text-selection-authority"}}},
		{Phase: "triage", Verdict: cyclestate.VerdictFAIL, AbortReason: cyclestate.CycleTerminationTriageClaimFailed, Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeTriageScopeUnanswered, Subject: "goal-text-has-no-selection-authority"}}},
	})

	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 1757, io.Discard)
	if in.SystemLevel || in.Refusal != cyclestate.DiagCodeTriageScopeUnanswered {
		t.Fatalf("inputs = SystemLevel %v, Refusal %q; the lane's unanswered scope is a task-level refusal, never the pipeline's", in.SystemLevel, in.Refusal)
	}
	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}

	item := findItemFile(t, filepath.Join(root, ".evolve", "inbox"), "goal-text-has-no-selection-authority")
	if got := itemFailureCount(t, item); got != 1 {
		t.Fatalf("failure_count = %d, want 1: cycle 1757 left its item at 0, first in line for the next wave", got)
	}
}
