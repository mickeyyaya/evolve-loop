package panestream

import "testing"

// LivenessState renders as the word the Signal Center's pane.liveness event
// carries in fields.state (ADR-0101 S3): one spelling, owned by the vocabulary.
func TestLivenessState_StringNamesEveryState(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		state LivenessState
		want  string
	}{
		{LivenessIdle, "idle"},
		{LivenessBusyButStagnant, "busy-stagnant"},
		{LivenessConverging, "converging"},
		{LivenessHung, "hung"},
		{LivenessExhausted, "exhausted"},
		{LivenessState(0), "unknown"},
		{LivenessState(99), "unknown"},
	} {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("LivenessState(%d).String() = %q, want %q", int(tc.state), got, tc.want)
		}
	}
}
