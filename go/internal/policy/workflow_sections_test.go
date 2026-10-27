package policy

import (
	"reflect"
	"testing"
)

func TestWorkflowPolicy_ASectionReaderLeavesItsInputConfigUnchanged(t *testing.T) {
	t.Parallel()
	in := Policy{}.defaultWorkflowConfig()
	before := map[string]float64{"trivial": 1.0, "small": 1.0, "medium": 1.25, "large": 1.5}
	w := WorkflowPolicy{SizeBudgetMultipliers: map[string]float64{"large": 3.0}}

	out := w.withRemediation(in)

	if !reflect.DeepEqual(in.SizeBudgetMultipliers, before) {
		t.Fatalf("the input config changed: %v, want %v", in.SizeBudgetMultipliers, before)
	}
	if out.SizeBudgetMultipliers["large"] != 3.0 || out.SizeBudgetMultipliers["medium"] != 1.25 {
		t.Fatalf("the output merges the override over the defaults; got %v", out.SizeBudgetMultipliers)
	}
}
