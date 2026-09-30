package core

import (
	"reflect"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// canonicalRetryHooks is the full hook set, including the quota-checkpoint
// classifier shared by fresh and resumed dispatch. Adding a hook to the
// dispatch loop without adding it here (and to retryOpts) fails this table —
// which is the whole point of the pin.
var canonicalRetryHooks = []string{
	"backfill",
	"optionalInfraSkip",
	"postShipObserverSkip",
	"quotaExhausted",
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

func TestRetryOpts_EnumeratesEveryDispatchHook(t *testing.T) {
	got := retryOptsHookNames(t, retryOpts{})
	if !reflect.DeepEqual(got, canonicalRetryHooks) {
		t.Fatalf("retryOpts hook fields = %v, want exactly %v\n"+
			"a NEW dispatch hook must be registered in retryOpts (and listed in canonicalRetryHooks) "+
			"or the batch path silently diverges again",
			got, canonicalRetryHooks)
	}
}

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
	if enabled["quotaExhausted"] {
		t.Error("evaluate batch has no partial-batch quota checkpoint and must retain its existing disposition")
	}
}

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
	if wResp.Verdict != VerdictSKIPPED || cResp.Verdict != VerdictSKIPPED {
		t.Errorf("verdicts = wrapper %q / shared-core %q, want both %q",
			wResp.Verdict, cResp.Verdict, VerdictSKIPPED)
	}
	if wAttempts != cAttempts {
		t.Errorf("attempts = wrapper %d / shared-core %d — dispatchRunnerWithRetry must DELEGATE "+
			"to retryPhaseRunner, not keep a second hand-maintained loop", wAttempts, cAttempts)
	}
}
