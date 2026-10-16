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
	want := recurrence.DefaultEscalationPolicy().Target(0.70, 3)
	if len(res.Bumped) != 1 || readWeight(t, path) != want {
		t.Fatalf("cycle-0 pass applied nothing: Bumped=%v weight=%v, want 1 bump to %v — the zero-value stamp read swallowed the intent", res.Bumped, readWeight(t, path), want)
	}

	second, err := recurrence.ApplyBoundary(opts(root, 0, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (second): %v", err)
	}
	if len(second.Bumped) != 0 || len(second.Skipped) != 0 || readWeight(t, path) != want {
		t.Fatalf("second cycle-0 pass was not a stamp skip: Bumped=%v Skipped=%v weight=%v", second.Bumped, second.Skipped, readWeight(t, path))
	}
}
