package policy

import "testing"

func TestRetryPolicy_UnmarshalJSONRefusesAValueThatIsNotARetryBlock(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"not an object":    `[1]`,
		"wrong field type": `{"phase_max_attempts":"two"}`,
	} {
		var r RetryPolicy
		if err := r.UnmarshalJSON([]byte(text)); err == nil {
			t.Errorf("%s: UnmarshalJSON(%s) = nil, want an error", name, text)
		}
	}
}

func TestRetryPolicy_UnmarshalJSONRecordsWhichRoundsFieldWasSet(t *testing.T) {
	t.Parallel()
	var set, unset RetryPolicy
	if err := set.UnmarshalJSON([]byte(`{"floor_convergence_rounds":0}`)); err != nil {
		t.Fatal(err)
	}
	if err := unset.UnmarshalJSON([]byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if !set.floorConvergenceRoundsSet || unset.floorConvergenceRoundsSet {
		t.Errorf("set = %v, unset = %v, want an explicit zero told apart from an absent field", set.floorConvergenceRoundsSet, unset.floorConvergenceRoundsSet)
	}
}
