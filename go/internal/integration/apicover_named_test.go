package integration

import "testing"

func TestExportedSymbolsAreNamedByAnExecutedAssertion(t *testing.T) {
	triggers := []FlushTrigger{TriggerNone, TriggerWaveEnd, TriggerCapReached, TriggerMaxAge, TriggerUnblocksQueuedWork, TriggerRiskyAlone}
	seen := map[FlushTrigger]bool{}
	for _, trigger := range triggers {
		if seen[trigger] {
			t.Errorf("trigger %q repeats; each trigger must be a distinct value", trigger)
		}
		seen[trigger] = true
	}
	if TriggerNone != "" {
		t.Errorf("TriggerNone = %q, want the empty trigger", TriggerNone)
	}

	policy := DefaultFlushPolicy()
	var decision FlushDecision = DecideFlush([]Deliverable{{ID: "a"}}, true, policy, decisionTime)
	if decision.Trigger != TriggerWaveEnd {
		t.Errorf("decision.Trigger = %q, want %q", decision.Trigger, TriggerWaveEnd)
	}
	if MinCap > MaxCap || DefaultCap < MinCap || DefaultCap > MaxCap {
		t.Errorf("bounds min %d default %d max %d are inconsistent", MinCap, DefaultCap, MaxCap)
	}
	if HalveCap(GrowCap(MinCap)) != MinCap {
		t.Errorf("HalveCap(GrowCap(MinCap)) = %d, want %d", HalveCap(GrowCap(MinCap)), MinCap)
	}
	if DefaultMaxAgeMinutes != policy.MaxAgeMinutes {
		t.Errorf("DefaultMaxAgeMinutes = %d, policy %d", DefaultMaxAgeMinutes, policy.MaxAgeMinutes)
	}
}
