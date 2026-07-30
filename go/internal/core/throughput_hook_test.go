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

// TestIsShippingVerdict_WholeOutcomeVocabulary walks every label the
// ADR-0079 outcome vocabulary can put in CycleResult.FinalVerdict. The
// allowlist shape is load-bearing: SKIPPED_UNKNOWN once fell through a
// denylist-shaped breaker, so an unrecognised label must classify as
// NON-shipping rather than defaulting to "shipped".
func TestIsShippingVerdict_WholeOutcomeVocabulary(t *testing.T) {
	tests := []struct {
		verdict string
		want    bool
	}{
		{VerdictPASS, true},
		{CycleOutcomeShippedViaBuild, true},
		{VerdictFAIL, false},
		{VerdictWARN, false},
		{CycleOutcomeSkippedAuditAdvisory, false},
		{CycleOutcomeSkippedUnknown, false},
		{"SKIPPED", false},
		{"", false},        // never recorded — must not read as shipped
		{"pass", false},    // case matters; the labels are exact
		{"SHIPPED", false}, // a plausible future label nobody classified
	}
	for _, tt := range tests {
		if got := IsShippingVerdict(tt.verdict); got != tt.want {
			t.Errorf("IsShippingVerdict(%q) = %v, want %v", tt.verdict, got, tt.want)
		}
	}
}

// TestIsShippingVerdict_IsTheOneDefinitionShippedOutcomeUses pins the
// coupling that justifies exporting it: with HEAD moved, shippedOutcome must
// agree with IsShippingVerdict on EVERY label. cmd/evolve's non-progress
// breaker consumes the same function negated, so a divergence here is a
// divergence between the throughput window and the breaker.
func TestIsShippingVerdict_IsTheOneDefinitionShippedOutcomeUses(t *testing.T) {
	for _, v := range []string{
		VerdictPASS, VerdictFAIL, VerdictWARN,
		CycleOutcomeShippedViaBuild, CycleOutcomeSkippedAuditAdvisory,
		CycleOutcomeSkippedUnknown, "SKIPPED", "", "SHIPPED",
	} {
		if got, want := shippedOutcome(v, "aaa", "bbb"), IsShippingVerdict(v); got != want {
			t.Errorf("verdict %q: shippedOutcome(HEAD moved) = %v but IsShippingVerdict = %v — the vocabulary has forked", v, got, want)
		}
	}
}
