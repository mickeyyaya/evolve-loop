package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestWaveLatencyPolicy_BoundsDefaultAndComeFromTheFile(t *testing.T) {
	var unset policy.Policy
	if got := unset.WaveLatencyMinLiveShare(); got != 0.8 {
		t.Errorf("WaveLatencyMinLiveShare default = %v, want 0.8", got)
	}
	if got := unset.WaveLatencyMaxGateShare(); got != 0.25 {
		t.Errorf("WaveLatencyMaxGateShare default = %v, want 0.25", got)
	}

	set, err := policy.Parse([]byte(`{"wave_latency":{"min_live_share":0.5,"max_gate_share":0.75}}`))
	if err != nil {
		t.Fatal(err)
	}
	var section *policy.WaveLatencyPolicy = set.WaveLatency
	if section == nil || section.MinLiveShare != 0.5 || section.MaxGateShare != 0.75 {
		t.Fatalf("WaveLatency = %+v, want the two bounds of the file", section)
	}
	if set.WaveLatencyMinLiveShare() != 0.5 || set.WaveLatencyMaxGateShare() != 0.75 {
		t.Errorf("bounds = %v, %v, want 0.5, 0.75", set.WaveLatencyMinLiveShare(), set.WaveLatencyMaxGateShare())
	}
}
