package dispositionrouter_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
)

// TestExportedVocabularyNamed pins the exported constants the behavioural tests
// reference only indirectly, so apicover sees every export asserted.
func TestExportedVocabularyNamed(t *testing.T) {
	t.Parallel()
	if dispositionrouter.RecurrenceConsoleFloor != 3 {
		t.Fatalf("RecurrenceConsoleFloor = %d, want 3", dispositionrouter.RecurrenceConsoleFloor)
	}
	if dispositionrouter.PendingActionsFile != "pending-actions.jsonl" {
		t.Fatalf("PendingActionsFile = %q", dispositionrouter.PendingActionsFile)
	}
	if got, want := dispositionrouter.PendingActionsPath("d"), filepath.Join("d", dispositionrouter.PendingActionsFile); got != want {
		t.Fatalf("PendingActionsPath = %q, want %q", got, want)
	}
	var d dispositionrouter.Decision = dispositionrouter.Decide(dispositionrouter.GuardAbortClass, 1, "")
	if d.Route != dispositionrouter.RouteConsole || d.Reason == "" || !d.Forced {
		t.Fatalf("Decision = %+v", d)
	}
	in := dispositionrouter.Intent{Cycle: 1, Pattern: "p", ItemID: "i", Action: dispositionrouter.ActionAutofile, Route: dispositionrouter.RouteQueue, Recurrence: 2, Weight: 0.5, Reason: "r"}
	if in.Action != "autofile" || in.Route != "queue" {
		t.Fatalf("Intent vocabulary drifted: %+v", in)
	}
}
