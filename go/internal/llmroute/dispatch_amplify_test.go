package llmroute

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// Test Amplification (cycle 435, black-box adversarial pass on top of the
// TDD-authored dispatch_test.go). These tests were designed from the
// contract only (Plan/DispatchResult shapes + Dispatch/ChainFor signatures,
// pinned by the pre-existing RED-turned-GREEN suite) without reading
// dispatch.go's actual walk implementation, per the amplifier's black-box
// mandate. They target basic/edge/null/negative/large-scale inputs the
// original suite didn't cover: the full default trigger set, chain length
// != 2, empty input, and concurrent independent use.

// TestDispatch_EmptyCandidatesNeverCallsLaunchOrClaimsSuccess (null/empty):
// a Plan with zero candidates has nothing to dispatch. Rule 12 (fail loudly)
// means Dispatch must surface an error rather than silently returning a
// zero-value DispatchResult{CLI:"", Err:nil} that a caller could mistake for
// "dispatched successfully to CLI ”" -- a silent-success gap would be far
// worse than a stopped chain, since the advisor/runner would think a plan
// or phase response actually exists.
func TestDispatch_EmptyCandidatesNeverCallsLaunchOrClaimsSuccess(t *testing.T) {
	sl := &scriptedLaunch{seq: []scriptedAttempt{{exitCode: 0, err: nil}}}
	plan := Plan{Candidates: nil, Triggers: defaultFallbackOnExit}

	got := Dispatch(plan, sl.launch)

	if len(sl.calls) != 0 {
		t.Errorf("Dispatch: launch called %v with zero candidates, want no calls", sl.calls)
	}
	if got.Err == nil {
		t.Errorf("Dispatch: empty-candidate Plan returned Err=nil CLI=%q -- a caller checking only Err would wrongly treat this as a successful dispatch to no CLI at all (Rule 12: fail loudly, don't silently claim success)", got.CLI)
	}
}

// TestDispatch_AllDefaultTriggerCodesAdvanceChain (basic, table-driven): the
// cycle-435 goal names five standard fallback triggers [80 81 85 124 127].
// The pre-existing suite only exercises 81 end-to-end; this pins the other
// four so a future edit to the trigger set (or a partial implementation that
// special-cased 81) can't silently regress the other codes.
func TestDispatch_AllDefaultTriggerCodesAdvanceChain(t *testing.T) {
	for _, code := range defaultFallbackOnExit {
		code := code
		t.Run(fmt.Sprintf("exit=%d", code), func(t *testing.T) {
			plan := Plan{Candidates: []string{"primary-cli", "fallback-cli"}, Triggers: defaultFallbackOnExit}
			sl := &scriptedLaunch{seq: []scriptedAttempt{
				{exitCode: code, err: fmt.Errorf("primary-cli: launch exit=%d", code)},
				{exitCode: 0, err: nil},
			}}

			got := Dispatch(plan, sl.launch)

			if got.Err != nil {
				t.Fatalf("Dispatch: exit=%d did not fall back, got err=%v", code, got.Err)
			}
			if got.CLI != "fallback-cli" {
				t.Errorf("Dispatch: exit=%d CLI=%q, want fallback-cli", code, got.CLI)
			}
			want := []string{"primary-cli", "fallback-cli"}
			if !reflect.DeepEqual(sl.calls, want) {
				t.Errorf("Dispatch: exit=%d launched %v, want %v", code, sl.calls, want)
			}
		})
	}
}

// TestDispatch_NonTriggerBoundaryCodesNeverReroute (negative, table-driven,
// boundary values): codes immediately adjacent to the default trigger set
// (79/82, 84/86, 123/126, 128) plus a couple of common "genuine failure"
// codes (1, 2) must NEVER advance the chain -- a fuzzy "is it close to a
// trigger" implementation would leak on these boundaries even though the
// existing suite's single non-trigger case (exit=1) passes.
func TestDispatch_NonTriggerBoundaryCodesNeverReroute(t *testing.T) {
	for _, code := range []int{79, 82, 84, 86, 123, 126, 128, 1, 2} {
		code := code
		t.Run(fmt.Sprintf("exit=%d", code), func(t *testing.T) {
			plan := Plan{Candidates: []string{"primary-cli", "fallback-cli"}, Triggers: defaultFallbackOnExit}
			sl := &scriptedLaunch{seq: []scriptedAttempt{
				{exitCode: code, err: fmt.Errorf("primary-cli: real failure exit=%d", code)},
			}}

			got := Dispatch(plan, sl.launch)

			if got.Err == nil {
				t.Fatalf("Dispatch: exit=%d expected the non-trigger error to surface, got nil", code)
			}
			if len(sl.calls) != 1 {
				t.Errorf("Dispatch: exit=%d launched %v, want exactly 1 call (must never reroute a boundary non-trigger code)", code, sl.calls)
			}
		})
	}
}

// TestDispatch_SingleCandidateTriggerExitStillDegrades (edge): a one-element
// chain (no fallback declared) that hits a trigger exit has nowhere to
// advance to. Dispatch must still surface the error with exactly one
// attempt recorded -- not hang, not panic on an out-of-range fallback index,
// and not misreport success.
func TestDispatch_SingleCandidateTriggerExitStillDegrades(t *testing.T) {
	plan := Plan{Candidates: []string{"only-cli"}, Triggers: defaultFallbackOnExit}
	sl := &scriptedLaunch{seq: []scriptedAttempt{{exitCode: 81, err: errors.New("only-cli: exit=81")}}}

	got := Dispatch(plan, sl.launch)

	if got.Err == nil {
		t.Fatalf("Dispatch: single-candidate trigger exit expected an error (no fallback exists), got nil")
	}
	if len(sl.calls) != 1 {
		t.Errorf("Dispatch: launched %v, want exactly 1 call (nothing to fall back to)", sl.calls)
	}
}

// TestDispatch_LongChainWalksEveryCandidateInOrder (large-scale): a 25-CLI
// chain where every candidate but the last exhausts a trigger exit. Proves
// the walk isn't hardcoded to a 1-or-2-candidate assumption and doesn't
// short-circuit or reorder partway through a long chain.
func TestDispatch_LongChainWalksEveryCandidateInOrder(t *testing.T) {
	const n = 25
	candidates := make([]string, n)
	seq := make([]scriptedAttempt, n)
	for i := 0; i < n; i++ {
		candidates[i] = fmt.Sprintf("cli-%02d", i)
		if i < n-1 {
			seq[i] = scriptedAttempt{exitCode: 81, err: fmt.Errorf("cli-%02d: exit=81", i)}
		} else {
			seq[i] = scriptedAttempt{exitCode: 0, err: nil}
		}
	}
	plan := Plan{Candidates: candidates, Triggers: defaultFallbackOnExit}
	sl := &scriptedLaunch{seq: seq}

	got := Dispatch(plan, sl.launch)

	if got.Err != nil {
		t.Fatalf("Dispatch: long chain expected the final candidate to succeed, got err=%v", got.Err)
	}
	if got.CLI != candidates[n-1] {
		t.Errorf("Dispatch: CLI=%q, want %q (the last candidate)", got.CLI, candidates[n-1])
	}
	if !reflect.DeepEqual(sl.calls, candidates) {
		t.Errorf("Dispatch: launched %v, want every candidate in order %v", sl.calls, candidates)
	}
	if !reflect.DeepEqual(got.Attempts, candidates) {
		t.Errorf("Dispatch: Attempts=%v, want %v", got.Attempts, candidates)
	}
}

// TestDispatch_ConcurrentIndependentCallsStayIsolated (concurrency, the
// "go test -race green" requirement the cycle-435 goal names explicitly):
// many goroutines call Dispatch concurrently, each with its own Plan and
// scriptedLaunch closure. Dispatch must not share any mutable state across
// calls (e.g. a package-level counter or cache) -- every goroutine's result
// must reflect only its own scripted sequence.
func TestDispatch_ConcurrentIndependentCallsStayIsolated(t *testing.T) {
	const workers = 20
	var wg sync.WaitGroup
	errs := make([]error, workers)
	clis := make([]string, workers)

	for w := 0; w < workers; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			primary := fmt.Sprintf("primary-%d", w)
			fallback := fmt.Sprintf("fallback-%d", w)
			plan := Plan{Candidates: []string{primary, fallback}, Triggers: []int{81}}
			sl := &scriptedLaunch{seq: []scriptedAttempt{
				{exitCode: 81, err: fmt.Errorf("%s: exit=81", primary)},
				{exitCode: 0, err: nil},
			}}
			got := Dispatch(plan, sl.launch)
			errs[w] = got.Err
			clis[w] = got.CLI
		}()
	}
	wg.Wait()

	for w := 0; w < workers; w++ {
		if errs[w] != nil {
			t.Errorf("worker %d: unexpected error %v", w, errs[w])
		}
		want := fmt.Sprintf("fallback-%d", w)
		if clis[w] != want {
			t.Errorf("worker %d: CLI=%q, want %q (cross-goroutine contamination)", w, clis[w], want)
		}
	}
}
