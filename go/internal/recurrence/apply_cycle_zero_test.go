package recurrence_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func TestApplyBoundary_CycleZeroAppliesStagedIntents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := seedItem(t, filepath.Join(root, "inbox"), "starved", 0.70)
	stage(t, root, dispositionrouter.ActionEscalate, "starved", "pattern:z", 3, 0.70)

	res, err := recurrence.ApplyBoundary(opts(root, 0, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if len(res.Escalated) != 1 || readWeight(t, path) != 0.70 {
		t.Fatalf("cycle-0 pass applied nothing: Escalated=%v weight=%v, want 1 escalation and the weight as filed — the zero-value stamp read swallowed the intent", res.Escalated, readWeight(t, path))
	}

	second, err := recurrence.ApplyBoundary(opts(root, 0, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (second): %v", err)
	}
	if len(second.Escalated) != 0 || len(second.Skipped) != 0 || readWeight(t, path) != 0.70 {
		t.Fatalf("second cycle-0 pass was not a stamp skip: Escalated=%v Skipped=%v weight=%v", second.Escalated, second.Skipped, readWeight(t, path))
	}
}
