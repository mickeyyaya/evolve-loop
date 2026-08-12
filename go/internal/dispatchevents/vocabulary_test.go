package dispatchevents

// vocabulary_test.go — the abnormal-events vocabulary is shared with a SECOND
// writer (internal/phasewatchdog), so membership has to be askable rather than
// reproducible: the moment the other writer keeps its own copy of the list, the
// two drift and records that parse become invisible to typed readers.

import "testing"

func TestIsKnownEventType_IsTheClosedVocabulary(t *testing.T) {
	t.Parallel()
	// Every declared type is known.
	for _, et := range []EventType{
		EventCounterNonAdvance, EventCircuitBreakerTripped, EventVerifyFailed,
		EventClassification, EventGoalStallEscalated, EventStallDetected,
	} {
		if !IsKnownEventType(et) {
			t.Errorf("%q is declared but not in the vocabulary — the sibling writer cannot assert its own records are readable", et)
		}
	}
	// And an invented one is not, or the predicate would be decoration.
	for _, et := range []EventType{"", "stall", "STALL-DETECTED", "made-up"} {
		if IsKnownEventType(et) {
			t.Errorf("%q was accepted — a predicate that says yes to everything cannot catch drift", et)
		}
	}
	// EventStallDetected specifically: the watchdog's record type, declared
	// here rather than in the watchdog so one home owns the vocabulary.
	if EventStallDetected != "stall-detected" {
		t.Errorf("EventStallDetected = %q — the wire value is consumed by existing operator jq filters and must not change", EventStallDetected)
	}
}
