package recurrence_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// TestApplyBoundary_CycleZeroAppliesStagedIntents pins the 2026-08-10
// iteration-0 swallow: the per-cycle idempotency stamp compared
// `stamp[key] == opts.Cycle`, and a NEVER-applied key reads as the map zero
// value 0 — so the fleet boundary passes (cmd_loop.go dispatches with the
// 0-based pool iteration index as Cycle) silently skipped EVERY staged intent
// on iteration 0. Observed live the day failure_disposition.stage flipped to
// enforce: apply report {bumped:null,filed:null,skipped:null,shadow:false,
// cycle:0}, stamp {} — 5 intents pending since cycles 1225-1378, zero applied.
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

	// Idempotency must still hold AT cycle 0: a second pass in the same
	// iteration changes nothing. Skipped must be empty too — a stamp skip is a
	// silent continue, while a stamp-BYPASSING pass records the id in Skipped
	// via the never-lower branch, which would mask a dead stamp guard here.
	second, err := recurrence.ApplyBoundary(opts(root, 0, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (second): %v", err)
	}
	if len(second.Bumped) != 0 || len(second.Skipped) != 0 || readWeight(t, path) != want {
		t.Fatalf("second cycle-0 pass was not a stamp skip: Bumped=%v Skipped=%v weight=%v", second.Bumped, second.Skipped, readWeight(t, path))
	}
}
