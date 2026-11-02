package cyclespend

import (
	"testing"
	"time"
)

func TestSignalNames_AreTheSpellingThatTheReadersParse(t *testing.T) {
	t.Parallel()
	for got, want := range map[string]string{SignalCode: "CYCLE_SPEND_LIMIT", FieldDecision: "decision", ActionOver: "over"} {
		if got != want {
			t.Errorf("name = %q, want %q", got, want)
		}
	}
}

func TestDecision_ZeroIsNoLimit(t *testing.T) {
	t.Parallel()
	var d Decision = Judge(Spend{}, defaultLimits, ActionStop)
	if d != (Decision{Action: ActionNone}) {
		t.Errorf("Judge of no spend = %+v, want none and no limit", d)
	}
}

func TestVerdictOf_IsNamedForTheAPICoverGate(t *testing.T) {
	t.Parallel()
	var v Verdict = VerdictOf(nil, time.Time{}, true)
	if v.LLMMinutesToVerdict != 0 || v.FailPointMinutes != nil {
		t.Errorf("VerdictOf of no calls = %+v, want zero minutes and no fail point", v)
	}
}
