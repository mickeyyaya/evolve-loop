package core

// retry_opts_parity_test.go — RED contract for cycle-1166 Task 1
// (evaluate-batch-retry-parity, inbox weight 0.87).
//
// State of the world when this file was authored. The inbox item's FIRST half
// ("dispatchRunnerWithRetry is missing optionalInfraSkip + postShipObserverSkip")
// has ALREADY landed: evaluate_batch.go:110 calls both predicates, and
// evaluate_batch_retry_parity_test.go is green. What has NOT landed is the
// item's stated FIX — "extract the shared retry core … retryOpts is a small
// Strategy struct carrying the optional hooks" — and its second acceptance
// criterion, the PARITY PIN:
//
//	"a table test enumerating retryOpts hooks asserts the main loop passes the
//	 full set (a new hook added to cyclerun without registering in retryOpts
//	 fails compilation or the table)"
//
// Two copies of the retry loop that merely happen to agree today is exactly the
// replicated-beliefs disease the item cites: the NEXT hook added to
// cyclerun_dispatch.go silently misses the batch path again. The pin below is
// the structural guard that makes that impossible.
//
// RED today: `retryOpts`, `retryPhaseRunner`, `mainDispatchRetryOpts` and
// `evaluateBatchRetryOpts` do not exist, so this file does not compile. That is
// the intended RED — a compile failure naming the missing Strategy struct.
//
// Contract Builder must satisfy (names are load-bearing — this file binds them):
//
//	type retryOpts struct {
//	    backfill             func(...) ...   // nil ⇒ hook disabled
//	    optionalInfraSkip    func(...) ...
//	    shipRecovery         func(...) ...
//	    postShipObserverSkip func(...) ...
//	}
//	func (cr *cycleRun) retryPhaseRunner(phase Phase, req PhaseRequest, opts retryOpts) (PhaseResponse, int, error)
//	func (cr *cycleRun) mainDispatchRetryOpts() retryOpts    // the sequential loop's hook set
//	func (cr *cycleRun) evaluateBatchRetryOpts() retryOpts   // the batch path's subset
//
// Field SIGNATURES are deliberately NOT pinned (Builder owns those); only the
// field NAME SET and each constructor's enabled/disabled hooks are.

import (
	"reflect"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// canonicalRetryHooks is the full hook set the item names verbatim: "backfill,
// optionalInfraSkip, shipRecovery, postShipObserverSkip". Adding a hook to the
// dispatch loop without adding it here (and to retryOpts) fails this table —
// which is the whole point of the pin.
var canonicalRetryHooks = []string{
	"backfill",
	"optionalInfraSkip",
	"postShipObserverSkip",
	"shipRecovery",
}

// retryOptsHookNames reflects over a retryOpts value and returns its field
// names, sorted. Reflection (not a hand-maintained list inside the production
// struct) is what makes the pin non-gameable: a new field appears here whether
// or not the implementer remembers this test.
func retryOptsHookNames(t *testing.T, v any) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	if rt.Kind() != reflect.Struct {
		t.Fatalf("retryOpts must be a struct (a Strategy value), got kind %s", rt.Kind())
	}
	names := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		names = append(names, rt.Field(i).Name)
	}
	sort.Strings(names)
	return names
}

// enabledRetryHooks reports, per hook name, whether that hook is wired (non-nil)
// on the given retryOpts value. A nil func field means "this path does not run
// that hook" — the explicit, inspectable form of the divergence the item is about.
func enabledRetryHooks(t *testing.T, v any) map[string]bool {
	t.Helper()
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct {
		t.Fatalf("retryOpts must be a struct, got kind %s", rv.Kind())
	}
	out := make(map[string]bool, rv.NumField())
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Kind() != reflect.Func {
			t.Fatalf("retryOpts field %q must be a func (a Strategy hook), got kind %s",
				rv.Type().Field(i).Name, f.Kind())
		}
		out[rv.Type().Field(i).Name] = !f.IsNil()
	}
	return out
}

// retryOptsCycleRun builds a cycleRun wired well enough for the two hook-set
// constructors, reusing this package's existing parity harness.
func retryOptsCycleRun(t *testing.T) *cycleRun {
	t.Helper()
	runner := &alwaysFailRunner{name: "evaluator", err: ErrArtifactTimeout}
	o := retryParityOrchestrator(t, runner, "evaluator", phasespec.PhaseSpec{Optional: true})
	return retryParityCycleRun(o, t)
}

// TestRetryOpts_EnumeratesEveryDispatchHook is the PARITY PIN itself (AC-2).
// The retryOpts Strategy struct must carry EXACTLY the canonical hook set: a
// hook added to the sequential dispatch loop but not registered in retryOpts
// leaves the batch path silently behind — the defect this item exists to make
// structurally impossible.
func TestRetryOpts_EnumeratesEveryDispatchHook(t *testing.T) {
	got := retryOptsHookNames(t, retryOpts{})
	if !reflect.DeepEqual(got, canonicalRetryHooks) {
		t.Fatalf("retryOpts hook fields = %v, want exactly %v\n"+
			"a NEW dispatch hook must be registered in retryOpts (and listed in canonicalRetryHooks) "+
			"or the batch path silently diverges again",
			got, canonicalRetryHooks)
	}
}

// TestMainDispatchRetryOpts_PassesTheFullHookSet — AC-2's positive half: the
// sequential loop is the reference implementation, so it must wire EVERY hook.
func TestMainDispatchRetryOpts_PassesTheFullHookSet(t *testing.T) {
	cr := retryOptsCycleRun(t)
	enabled := enabledRetryHooks(t, cr.mainDispatchRetryOpts())
	for _, hook := range canonicalRetryHooks {
		if !enabled[hook] {
			t.Errorf("mainDispatchRetryOpts() left hook %q nil — the sequential loop is the "+
				"reference set and must pass all of %v", hook, canonicalRetryHooks)
		}
	}
}

// TestEvaluateBatchRetryOpts_WiresBothSkipsButNotShipRecovery is the NEGATIVE /
// anti-widen twin. The item is explicit that the batch passes "the
// evaluate-appropriate subset (ship recovery disabled)" — so a degenerate
// "just hand the batch the full set" implementation must FAIL here, while the
// two skip predicates that motivated the item must be present.
func TestEvaluateBatchRetryOpts_WiresBothSkipsButNotShipRecovery(t *testing.T) {
	cr := retryOptsCycleRun(t)
	enabled := enabledRetryHooks(t, cr.evaluateBatchRetryOpts())

	for _, hook := range []string{"optionalInfraSkip", "postShipObserverSkip"} {
		if !enabled[hook] {
			t.Errorf("evaluateBatchRetryOpts() left hook %q nil — this is the exact parity gap "+
				"the item was filed for; the batch path must run it", hook)
		}
	}
	if enabled["shipRecovery"] {
		t.Error("evaluateBatchRetryOpts() wired shipRecovery — the batch path must NOT run ship " +
			"recovery (evaluate phases never ship); widening the subset is the anti-goal")
	}
}

// TestDispatchRunnerWithRetry_DelegatesToTheSharedRetryCore proves the
// extraction actually HAPPENED rather than retryOpts being a decorative struct
// bolted beside two still-duplicated loops. dispatchRunnerWithRetry must produce
// the same outcome as calling the shared core directly with the batch hook set —
// for the exact scenario the item names (an optional phase exhausting retries on
// an infra timeout degrades instead of aborting the batch).
func TestDispatchRunnerWithRetry_DelegatesToTheSharedRetryCore(t *testing.T) {
	viaWrapper := func() (PhaseResponse, int, error) {
		runner := &alwaysFailRunner{name: "evaluator", err: ErrArtifactTimeout}
		o := retryParityOrchestrator(t, runner, "evaluator", phasespec.PhaseSpec{Optional: true})
		cr := retryParityCycleRun(o, t)
		return cr.dispatchRunnerWithRetry(Phase("evaluator"), PhaseRequest{})
	}
	viaCore := func() (PhaseResponse, int, error) {
		runner := &alwaysFailRunner{name: "evaluator", err: ErrArtifactTimeout}
		o := retryParityOrchestrator(t, runner, "evaluator", phasespec.PhaseSpec{Optional: true})
		cr := retryParityCycleRun(o, t)
		return cr.retryPhaseRunner(Phase("evaluator"), PhaseRequest{}, cr.evaluateBatchRetryOpts())
	}

	wResp, wAttempts, wErr := viaWrapper()
	cResp, cAttempts, cErr := viaCore()

	if wErr != nil || cErr != nil {
		t.Fatalf("optional off-floor phase exhausting infra retries must degrade (err==nil); "+
			"wrapper err=%v, shared-core err=%v", wErr, cErr)
	}
	if wResp.Verdict != VerdictWARN || cResp.Verdict != VerdictWARN {
		t.Errorf("verdicts = wrapper %q / shared-core %q, want both %q",
			wResp.Verdict, cResp.Verdict, VerdictWARN)
	}
	if wAttempts != cAttempts {
		t.Errorf("attempts = wrapper %d / shared-core %d — dispatchRunnerWithRetry must DELEGATE "+
			"to retryPhaseRunner, not keep a second hand-maintained loop", wAttempts, cAttempts)
	}
}
