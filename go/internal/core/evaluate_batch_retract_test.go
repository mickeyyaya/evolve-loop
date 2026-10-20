package core

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func retractBatchRun(t *testing.T, retract bool, order ...string) *cycleRun {
	t.Helper()
	cat, warnings := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "test-amplification", Role: "evaluate", WritesSource: true, RetractOnFail: retract},
		{Name: "adversarial-review", Role: "evaluate"},
		{Name: "perf-review", Role: "evaluate"},
	})
	if len(warnings) != 0 {
		t.Fatalf("catalog: %v", warnings)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithCatalog(cat))
	o.cfg.Order = order
	return &cycleRun{o: o}
}

func TestEvaluateBatchAt_ARetractOnFailPhaseNeverRunsBesideAnotherPhase(t *testing.T) {
	t.Parallel()
	if got := retractBatchRun(t, false, "build", "test-amplification", "adversarial-review", "audit").evaluateBatchAt("test-amplification"); len(got) != 2 {
		t.Fatalf("precondition: two evaluate phases after build batch, got %v", got)
	}
	for _, order := range [][]string{
		{"build", "test-amplification", "adversarial-review", "audit"},
		{"build", "adversarial-review", "test-amplification", "audit"},
	} {
		cr := retractBatchRun(t, true, order...)
		for _, p := range order[1:3] {
			if got := cr.evaluateBatchAt(Phase(p)); got != nil {
				t.Errorf("order %v: evaluateBatchAt(%s) = %v, want nil: a sibling's read-only fence can restore a retracted test (ADR-0149)", order, p, got)
			}
		}
	}
	cr := retractBatchRun(t, true, "build", "test-amplification", "adversarial-review", "perf-review", "audit")
	if got := cr.evaluateBatchAt("adversarial-review"); !reflect.DeepEqual(got, []Phase{"adversarial-review", "perf-review"}) {
		t.Errorf("evaluateBatchAt(adversarial-review) = %v, want the two read-only phases after the retracting one", got)
	}
}
