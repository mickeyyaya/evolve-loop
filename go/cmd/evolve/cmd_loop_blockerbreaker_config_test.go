package main

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestBlockerBreakerConfig_MapsEachCeilingAcksAndDeferrals(t *testing.T) {
	t.Parallel()
	th := policy.FailureThresholds{GuardClassHaltCeiling: 2, IdenticalFingerprintHaltCeiling: 3, UnexplainedFailuresHaltCeiling: 4, ConsecutiveFailuresHaltCeiling: 5}
	acked := map[string]bool{"fp-1": true}
	deferrals := []core.LaneDeferralRecord{{Cycle: 7}}
	got := blockerBreakerConfig(th, acked, deferrals)
	want := core.BlockerBreakerConfig{
		GuardClassCeiling: 2, IdenticalFingerprintCeiling: 3, UnexplainedCeiling: 4, ConsecutiveFailuresCeiling: 5,
		AckedFingerprints: acked, LaneDeferredCycles: core.LaneDeferredCycleSet(deferrals),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blockerBreakerConfig = %+v, want %+v", got, want)
	}
}
