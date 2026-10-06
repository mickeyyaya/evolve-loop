package fleet

import (
	"errors"
	"testing"
)

func TestResult_StatusIsTheOneLaneOutcomeClassifier(t *testing.T) {
	cases := []struct {
		name string
		r    Result
		want LaneStatus
	}{
		{"clean exit", Result{}, LaneOK},
		{"deferred exit", Result{ExitCode: ExitDeferred}, LaneDeferred},
		{"deferred code with a launch error", Result{ExitCode: ExitDeferred, Err: errors.New("spawn")}, LaneFailed},
		{"FAIL verdict", Result{ExitCode: 2}, LaneFailed},
		{"launch error", Result{ExitCode: -1, Err: errors.New("x")}, LaneFailed},
	}
	for _, tc := range cases {
		if got := tc.r.Status(); got != tc.want {
			t.Errorf("%s: Status() = %q, want %q", tc.name, got, tc.want)
		}
	}
	if ExitDeferred != 5 {
		t.Errorf("ExitDeferred = %d, want 5, the exit `evolve cycle run` gives a DEFERRED cycle", ExitDeferred)
	}
}
