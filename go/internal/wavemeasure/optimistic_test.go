package wavemeasure_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func TestCompute_TheOptimisticFailureRateIsFailuresOverAdmissions(t *testing.T) {
	t.Parallel()
	got := wavemeasure.Compute(wavemeasure.Facts{OptimisticAdmissions: 4, OptimisticFailures: 1}, at(0))
	if got.OptimisticAdmissions != 4 || got.OptimisticFailures != 1 || got.OptimisticFailureRate != 0.25 {
		t.Errorf("measures = %+v, want 1 of 4 optimistic admissions failed, rate 0.25", got)
	}
	if none := wavemeasure.Compute(wavemeasure.Facts{OptimisticFailures: 1}, at(0)); none.OptimisticFailureRate != 0 {
		t.Errorf("rate with no admission = %v, want 0", none.OptimisticFailureRate)
	}
}
