package inboxmover

import "testing"

func TestRetirementStates_CoverEveryRetiredStateOnce(t *testing.T) {
	want := map[string]bool{StateConsumed: true, StateQuarantine: true, StateProcessed: true, StateRejected: true, StateRetry: true}
	seen := map[string]bool{}
	for _, s := range retirementStates {
		if !want[s] || seen[s] {
			t.Fatalf("retirementStates carries %q twice or a non-retired state: %v", s, retirementStates)
		}
		seen[s] = true
	}
	if len(seen) != len(want) {
		t.Fatalf("retirementStates = %v, want every retired state: %v", retirementStates, want)
	}
	for _, live := range []string{StatePending, StateProcessing, StateUnknown} {
		if seen[live] {
			t.Fatalf("%q is live, not retired", live)
		}
	}
}
