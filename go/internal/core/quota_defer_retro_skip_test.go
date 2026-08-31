package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// RED contract for cycle-1585 task `quota-defer-short-circuits-retro`
// (instinct inst-L1582a), SUPERSEDED by the cycle-1587 fix for
// `pipeline-defect-pipeline-blocker-cycle1582`
// (.evolve/evals/pipeline-defect-pipeline-blocker-cycle1582.md). The
// all-families-quota-exhausted abort (cyclerun_dispatch.go:264-287) is a
// DEFERRED, resumable checkpoint, not a diagnosed phase failure: the
// quota-boundary checkpoint is already written and the loop exits rc=5 to be
// resumed after the quota resets.
//
// cycle-1585 only fixed the "no retro dispatch" half (recordFailureLearning
// still called recordFailedApproachState unconditionally before its
// ErrAllFamiliesExhausted short-circuit, so a FailedRecord + P0 carryover todo
// were still appended on every quota wall — the cycle-1582 dossier root
// cause: it queued a spurious `cycle-N-failed-scout` P0 todo that competed
// with real work every time the fleet hit quota). This RED contract closes
// the remaining half: the guard must short-circuit BEFORE
// recordFailedApproachState on the all-families-exhausted arm, not merely
// before the retro dispatch that follows it.
//
//	AC1 — retro runner is never called on the all-families-exhausted path
//	AC2 — CycleState.Phase / ActiveAgent are never mutated to "retro" there
//	AC3 — state.FailedAt / carryover-todo bookkeeping is NOT recorded for the
//	      all-families-exhausted arm (superseded from cycle-1585's "still
//	      recorded" expectation — a DEFERRED checkpoint is not a FAIL)
//	AC4 — a genuine non-quota failure still dispatches retro exactly once and
//	      still records FailedAt (unaffected by the guard)
//	AC5 — a multiply-wrapped sentinel is still matched (errors.Is, not ==),
//	      and skips bookkeeping too
//	AC6 — a single-family exit=85 attempt with a differently-shaped sibling
//	      (not the all-85 signature) is NOT the exhaustion arm at all, so it
//	      must learn exactly as before: unaffected by the guard

// allFamiliesExhaustedRun drives a full RunCycle to all-families quota
// exhaustion using the same harness as
// TestRunCycle_AllFamilies85_CheckpointsAndDefers: scout returns exit=85 on
// every attempt, so allFamiliesQuotaExhausted(attemptExits) is true and the
// dispatch seam constructs the %w-wrapped ErrAllFamiliesExhausted and calls
// cr.recordFailureLearning. Returns the storage fake and the runner map so
// callers can assert on what the production chain did.
func allFamiliesExhaustedRun(t *testing.T) (*fakeStorage, *fakeLedger, map[Phase]PhaseRunner, error) {
	t.Helper()
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error { return nil }

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(85), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("RunCycle: want error, got nil")
	}
	// Precondition, not the assertion under test: if this trips, the harness
	// is broken rather than the guard.
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want errors.Is ErrAllFamiliesExhausted (harness precondition)", err)
	}
	return st, led, runners, err
}

// AC1 + AC3: the counting fake proves the PRODUCTION dispatch chain never
// reaches the retro runner, AND — superseding cycle-1585's "bookkeeping still
// recorded" expectation — that a DEFERRED quota checkpoint records no
// FailedRecord and no P0 carryover todo either: it is not a diagnosed failure,
// so failure-learning must not fire at all on this arm (cycle-1582 dossier:
// the spurious `cycle-N-failed-scout` P0 todo competed with real work on every
// quota wall). NOT t.Parallel: swaps the package-level
// QuotaBoundaryCheckpointer hook.
func TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro(t *testing.T) {
	st, _, runners, _ := allFamiliesExhaustedRun(t)

	if calls := runners[PhaseRetro].(*fakeRunner).calls; calls != 0 {
		t.Errorf("retro runner calls=%d, want 0 — an all-families-quota-exhausted "+
			"(DEFERRED, resumable) abort must short-circuit before failure-learning "+
			"fires at all", calls)
	}
	if len(st.state.FailedAt) != 0 {
		t.Errorf("state.FailedAt=%+v, want empty — a DEFERRED quota checkpoint is not a "+
			"diagnosed failure, so recordFailedApproachState must not run on this arm", st.state.FailedAt)
	}
	for _, todo := range st.state.CarryoverTodos {
		if strings.Contains(todo.ID, "failed-scout") {
			t.Errorf("carryoverTodos=%+v, want no cycle-N-failed-scout entry — a DEFERRED "+
				"quota checkpoint must not queue a P0 failure-learning todo", st.state.CarryoverTodos)
		}
	}
}

// Regression reproducer for cycle-1582: all-family quota exhaustion is a
// DEFERRED checkpoint, not a diagnosed phase failure. The current tree skips
// retro but still creates failed-at and P0 carryover state before that guard.
func TestDispatch_AllFamiliesExhausted_NoFailureLearning(t *testing.T) {
	t.Run("no_FailedRecord_appended", func(t *testing.T) {
		st, _, _, _ := allFamiliesExhaustedRun(t)
		if got := len(st.state.FailedAt); got != 0 {
			t.Fatalf("state.FailedAt=%+v, want no FailedRecord for a DEFERRED quota checkpoint", st.state.FailedAt)
		}
	})

	t.Run("no_P0_carryover_todo_queued", func(t *testing.T) {
		st, _, _, _ := allFamiliesExhaustedRun(t)
		for _, todo := range st.state.CarryoverTodos {
			if strings.Contains(todo.ID, "failed-scout") {
				t.Fatalf("carryover todo=%+v, want no P0 failure-learning todo for a DEFERRED quota checkpoint", todo)
			}
		}
	})

	t.Run("retro_runner_never_invoked_for_learning", func(t *testing.T) {
		_, _, runners, _ := allFamiliesExhaustedRun(t)
		if got := runners[PhaseRetro].(*fakeRunner).calls; got != 0 {
			t.Fatalf("retro runner calls=%d, want 0 for a DEFERRED quota checkpoint", got)
		}
	})

	t.Run("dispatch_aborts_deferred_shaped", func(t *testing.T) {
		_, _, _, err := allFamiliesExhaustedRun(t)
		if !errors.Is(err, ErrAllFamiliesExhausted) {
			t.Fatalf("err=%v, want ErrAllFamiliesExhausted DEFERRED abort", err)
		}
	})

	t.Run("quota_boundary_ledger_entry_still_recorded", func(t *testing.T) {
		_, ledger, _, _ := allFamiliesExhaustedRun(t)
		found := false
		for _, entry := range ledger.entries {
			if entry.Kind == "all_families_exhausted" && entry.ExitCode == 85 {
				found = true
			}
		}
		if !found {
			t.Fatalf("ledger entries=%+v, want all_families_exhausted exit=85", ledger.entries)
		}
	})
}

// AC2: the deferral must leave the cycle state resumable at the exhausted
// phase. Mutating Phase/ActiveAgent to "retro" (and persisting it) makes
// `evolve loop --resume` resume into retro instead of the drained phase, so the
// guard must land before those writes — not merely skip the runner call.
func TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState(t *testing.T) {
	st, _, _, _ := allFamiliesExhaustedRun(t)

	for i, cs := range st.cycleStateLog {
		if cs.Phase == string(PhaseRetro) {
			t.Errorf("cycleStateLog[%d].Phase=%q, want never %q on the all-families-exhausted "+
				"path (resume must re-enter the exhausted phase)", i, cs.Phase, PhaseRetro)
		}
		if cs.ActiveAgent == string(PhaseRetro) {
			t.Errorf("cycleStateLog[%d].ActiveAgent=%q, want never %q on the all-families-exhausted path",
				i, cs.ActiveAgent, PhaseRetro)
		}
		for _, done := range cs.CompletedPhases {
			if done == string(PhaseRetro) {
				t.Errorf("cycleStateLog[%d].CompletedPhases=%v records retro as completed; "+
					"retro never ran on this path", i, cs.CompletedPhases)
			}
		}
	}
}

// AC4 (negative / anti-no-op): a plain non-quota dispatch failure is a real
// FAIL and MUST still reach retro exactly once. A fix that short-circuits
// unconditionally — or that keys off "any transient error" instead of the typed
// sentinel — passes AC1 and fails here.
func TestRunCycle_NonQuotaDispatchFailure_StillDispatchesRetroOnce(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	// exit=1 → errGenericExit, not transient and not quota: the loud-abort
	// branch (cyclerun_dispatch.go:366) records failure learning.
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(1), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("RunCycle: want error, got nil")
	}
	if errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v must NOT be ErrAllFamiliesExhausted (harness precondition)", err)
	}
	if calls := runners[PhaseRetro].(*fakeRunner).calls; calls != 1 {
		t.Errorf("retro runner calls=%d, want exactly 1 — a genuine non-quota failure must "+
			"still learn from retro", calls)
	}
	if len(st.state.FailedAt) == 0 {
		t.Errorf("state.FailedAt is empty on the ordinary failure path")
	}
}

// AC5 (edge / anti-gaming): the sentinel arrives %w-wrapped at least twice by
// the time it reaches recordFailureLearning (dispatch wraps it, then
// wrapCycleLevelError wraps that). A cheap `fl.Err == ErrAllFamiliesExhausted`
// identity check would pass AC1's happy path only by accident and break the
// moment another wrapper is added, so drive the chokepoint directly with a
// deliberately over-wrapped error.
func TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	state := State{LastCycleNumber: 41}
	cs := CycleState{CycleID: 42, Phase: string(PhaseScout), ActiveAgent: string(PhaseScout),
		WorkspacePath: t.TempDir()}
	result := CycleResult{Cycle: 42}
	timings := []phaseTimingEntry{}
	wrapped := fmt.Errorf("outer: %w",
		fmt.Errorf("phase scout: %w: every family in the fallback chain returned exit=85",
			ErrAllFamiliesExhausted))

	o.recordFailureLearning(context.Background(), failureLearningRequest{
		CycleRequest: CycleRequest{ProjectRoot: t.TempDir()},
		Cycle:        42,
		Failed:       PhaseScout,
		Err:          wrapped,
		Attempt:      2,
		State:        &state,
		CycleState:   &cs,
		Result:       &result,
		Timings:      &timings,
	})

	if calls := runners[PhaseRetro].(*fakeRunner).calls; calls != 0 {
		t.Errorf("retro runner calls=%d, want 0 — the guard must use errors.Is on the "+
			"wrapped sentinel, not an identity comparison", calls)
	}
	if cs.Phase == string(PhaseRetro) || cs.ActiveAgent == string(PhaseRetro) {
		t.Errorf("CycleState phase=%q agent=%q, want both left at the exhausted phase %q",
			cs.Phase, cs.ActiveAgent, PhaseScout)
	}
	if len(state.FailedAt) != 0 {
		t.Errorf("state.FailedAt=%+v, want empty — the multiply-wrapped sentinel must be "+
			"recognized BEFORE recordFailedApproachState runs, not only before the retro dispatch", state.FailedAt)
	}
}

// TestRecordFailureLearning_ShipExhausted_PreservesCoherenceCarrier ensures a
// deferred ship quota checkpoint remains out of failure learning while keeping
// its real dispatch error available to the ADR-0072 coherence floor.
func TestRecordFailureLearning_ShipExhausted_PreservesCoherenceCarrier(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	state := State{}
	cs := CycleState{CycleID: 42, WorkspacePath: t.TempDir()}
	result := CycleResult{Cycle: 42}
	timings := []phaseTimingEntry{}
	err := fmt.Errorf("phase ship: %w", ErrAllFamiliesExhausted)

	o.recordFailureLearning(context.Background(), failureLearningRequest{
		Cycle: 42, Failed: PhaseShip, Err: err, State: &state, CycleState: &cs,
		Result: &result, Timings: &timings,
	})

	if got, want := cs.ShipFailReasons, []string{err.Error()}; !reflect.DeepEqual(got, want) {
		t.Errorf("ShipFailReasons=%q, want %q", got, want)
	}
	if len(state.FailedAt) != 0 {
		t.Errorf("FailedAt=%+v, want empty for deferred quota exhaustion", state.FailedAt)
	}
}

// AC6: a single-family exit=85 attempt with a differently-shaped sibling (80,
// not 85) is NOT the all-families-exhausted signature — allFamiliesQuotaExhausted
// returns false, so the loud-abort path (cyclerun_dispatch.go:366) runs
// unchanged. The fix must be scoped to the typed ErrAllFamiliesExhausted
// sentinel, never to "any attempt exited 85" — otherwise this ordinary
// mixed-failure path would silently stop learning too.
func TestDispatch_SingleFamily85WithSibling_FailureLearningUnchanged(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &seqFailRunner{name: "scout", errs: []error{wrapTransient(85), wrapTransient(80), wrapTransient(80)}}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("RunCycle: want error, got nil")
	}
	if errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v must NOT be ErrAllFamiliesExhausted (harness precondition: mixed 85/80 is not the all-85 signature)", err)
	}
	if calls := runners[PhaseRetro].(*fakeRunner).calls; calls != 1 {
		t.Errorf("retro runner calls=%d, want exactly 1 — a single-family-85-with-sibling "+
			"failure must still learn normally, the guard is scoped to the all-85 signature only", calls)
	}
	found := false
	for _, todo := range st.state.CarryoverTodos {
		if strings.Contains(todo.ID, "failed-scout") {
			found = true
		}
	}
	if !found {
		t.Errorf("carryoverTodos=%+v, want the cycle-N-failed-scout entry — this is not the "+
			"DEFERRED arm, normal failure-learning bookkeeping must still fire", st.state.CarryoverTodos)
	}
	if len(st.state.FailedAt) == 0 {
		t.Errorf("state.FailedAt is empty, want the FailedRecord for the ordinary (non-quota-exhausted) failure path")
	}
}
