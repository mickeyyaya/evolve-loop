package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// RED contract for cycle-1585 task `quota-defer-short-circuits-retro`
// (instinct inst-L1582a). The all-families-quota-exhausted abort
// (cyclerun_dispatch.go:264-287) is a DEFERRED, resumable outcome: the
// checkpoint is already written and the loop exits rc=5 to be resumed after the
// quota resets. Dispatching retro on that path burns a phase's worth of budget
// against the same drained quota wall and delays the deferral.
//
// The guard at failure_learning.go:344 short-circuits only for
// `fl.Failed == PhaseRetro`; it has no clause for the typed
// ErrAllFamiliesExhausted sentinel, so the DEFERRED path falls through to the
// retro dispatch body. These tests pin the fix and its blast radius:
//
//	AC1 — retro runner is never called on the all-families-exhausted path
//	AC2 — CycleState.Phase / ActiveAgent are never mutated to "retro" there
//	AC3 — state.FailedAt bookkeeping is still recorded (the fix must not
//	      return before recordFailedApproachState)
//	AC4 — a genuine non-quota failure still dispatches retro exactly once
//	AC5 — a multiply-wrapped sentinel is still matched (errors.Is, not ==)

// allFamiliesExhaustedRun drives a full RunCycle to all-families quota
// exhaustion using the same harness as
// TestRunCycle_AllFamilies85_CheckpointsAndDefers: scout returns exit=85 on
// every attempt, so allFamiliesQuotaExhausted(attemptExits) is true and the
// dispatch seam constructs the %w-wrapped ErrAllFamiliesExhausted and calls
// cr.recordFailureLearning. Returns the storage fake and the runner map so
// callers can assert on what the production chain did.
func allFamiliesExhaustedRun(t *testing.T) (*fakeStorage, map[Phase]PhaseRunner) {
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
	return st, runners
}

// AC1 + AC3: the counting fake proves the PRODUCTION dispatch chain never
// reaches the retro runner (a green unit test on the guard helper alone already
// passed once in cycle-1582 while the wiring was still broken), and the
// deterministic bookkeeping the failure adapter depends on survives the fix.
// NOT t.Parallel: swaps the package-level QuotaBoundaryCheckpointer hook.
func TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro(t *testing.T) {
	st, runners := allFamiliesExhaustedRun(t)

	if calls := runners[PhaseRetro].(*fakeRunner).calls; calls != 0 {
		t.Errorf("retro runner calls=%d, want 0 — an all-families-quota-exhausted "+
			"(DEFERRED, resumable) abort must short-circuit before the retro dispatch "+
			"in recordFailureLearning", calls)
	}
	if len(st.state.FailedAt) == 0 {
		t.Errorf("state.FailedAt is empty, want the FailedRecord still appended — the " +
			"short-circuit must land AFTER recordFailedApproachState, not replace it")
	}
	found := false
	for _, todo := range st.state.CarryoverTodos {
		if strings.Contains(todo.ID, "failed-scout") {
			found = true
		}
	}
	if !found {
		t.Errorf("carryoverTodos=%+v, want the cycle-N-failed-scout entry — deterministic "+
			"failure learning must survive the retro short-circuit", st.state.CarryoverTodos)
	}
}

// AC2: the deferral must leave the cycle state resumable at the exhausted
// phase. Mutating Phase/ActiveAgent to "retro" (and persisting it) makes
// `evolve loop --resume` resume into retro instead of the drained phase, so the
// guard must land before those writes — not merely skip the runner call.
func TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState(t *testing.T) {
	st, _ := allFamiliesExhaustedRun(t)

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
	if len(state.FailedAt) == 0 {
		t.Errorf("state.FailedAt is empty, want the FailedRecord recorded before the short-circuit")
	}
}
