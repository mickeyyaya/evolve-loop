package core

import "testing"

// R9.1 (concurrency-factory plan): the throughput-recorder seam. The
// orchestrator records observed builder throughput (coverage floors passed
// per cycle) ONLY for cycles that actually shipped — that is what makes the
// window an honest capacity signal for the R9.2 clamp.

func TestShippedOutcome(t *testing.T) {
	tests := []struct {
		name              string
		verdict           string
		preHEAD, postHEAD string
		want              bool
	}{
		{"PASS with HEAD movement ships", VerdictPASS, "aaa", "bbb", true},
		{"PASS without HEAD movement does not ship", VerdictPASS, "aaa", "aaa", false},
		{"inline build-ship counts", CycleOutcomeShippedViaBuild, "aaa", "bbb", true},
		{"FAIL never ships", VerdictFAIL, "aaa", "bbb", false},
		{"SKIPPED_UNKNOWN never ships", CycleOutcomeSkippedUnknown, "aaa", "aaa", false},
		{"empty HEADs (git unavailable) are not shipped evidence", VerdictPASS, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shippedOutcome(tt.verdict, tt.preHEAD, tt.postHEAD); got != tt.want {
				t.Errorf("shippedOutcome(%q, %q, %q) = %v, want %v", tt.verdict, tt.preHEAD, tt.postHEAD, got, tt.want)
			}
		})
	}
}

// TestThroughputRecorderWired_Probe: nil seam (default) reports unwired;
// WithThroughputRecorder flips the probe — the composition-root wiring test
// in cmd/evolve asserts the production root passes it.
func TestThroughputRecorderWired_Probe(t *testing.T) {
	bare := &Orchestrator{}
	if bare.ThroughputRecorderWired() {
		t.Error("zero orchestrator must report recorder unwired")
	}
	o := &Orchestrator{}
	WithThroughputRecorder(func(*State, int, string) {})(o)
	if !o.ThroughputRecorderWired() {
		t.Error("WithThroughputRecorder did not wire the seam")
	}
}
