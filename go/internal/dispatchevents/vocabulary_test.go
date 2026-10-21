package dispatchevents

import "testing"

func TestIsKnownEventType_IsTheClosedVocabulary(t *testing.T) {
	t.Parallel()
	for _, et := range []EventType{
		EventCounterNonAdvance, EventCircuitBreakerTripped, EventVerifyFailed,
		EventClassification, EventGoalStallEscalated, EventStallDetected,
		EventPhaseOutputsSurveyed, EventVerifySingleFlightUnavailable,
	} {
		if !IsKnownEventType(et) {
			t.Errorf("%q is declared but not in the vocabulary — the sibling writer cannot assert its own records are readable", et)
		}
	}
	for _, et := range []EventType{"", "stall", "STALL-DETECTED", "made-up"} {
		if IsKnownEventType(et) {
			t.Errorf("%q was accepted — a predicate that says yes to everything cannot catch drift", et)
		}
	}
	if EventStallDetected != "stall-detected" {
		t.Errorf("EventStallDetected = %q — the wire value is consumed by existing operator jq filters and must not change", EventStallDetected)
	}
}
