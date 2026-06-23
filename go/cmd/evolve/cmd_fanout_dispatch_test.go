package main

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

// FanoutConfig replaces fanoutEnvConfig. These tests pin the built-in defaults
// and override behavior of policy.Policy.FanoutConfig().

func TestFanoutPolicyConfig_Defaults(t *testing.T) {
	pol := policy.Policy{}
	fc := pol.FanoutConfig()
	if fc.Concurrency != 2 {
		t.Errorf("Concurrency default = %d, want 2", fc.Concurrency)
	}
	if fc.TimeoutSecs != 0 {
		t.Errorf("TimeoutSecs default = %d, want 0 (package sentinel)", fc.TimeoutSecs)
	}
	if fc.CancelOnConsensus {
		t.Errorf("CancelOnConsensus default = true, want false")
	}
	if fc.TrackWorkers == nil || !*fc.TrackWorkers {
		t.Errorf("TrackWorkers default = nil or false, want true")
	}
	if fc.CachePrefixEnabled == nil || !*fc.CachePrefixEnabled {
		t.Errorf("CachePrefixEnabled default = nil or false, want true")
	}
}

func TestFanoutPolicyConfig_Override(t *testing.T) {
	tw := false
	pol := policy.Policy{
		Fanout: &policy.FanoutPolicy{
			Concurrency:  4,
			TimeoutSecs:  600,
			ConsensusK:   2,
			TrackWorkers: &tw,
		},
	}
	fc := pol.FanoutConfig()
	if fc.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", fc.Concurrency)
	}
	if fc.TimeoutSecs != 600 {
		t.Errorf("TimeoutSecs = %d, want 600", fc.TimeoutSecs)
	}
	if fc.ConsensusK != 2 {
		t.Errorf("ConsensusK = %d, want 2", fc.ConsensusK)
	}
	if fc.TrackWorkers == nil || *fc.TrackWorkers {
		t.Errorf("TrackWorkers = nil or true, want &false")
	}
}

func TestFanoutPolicyConfig_ConcurrencyFloor(t *testing.T) {
	// Concurrency < 1 falls back to the default 2.
	pol := policy.Policy{Fanout: &policy.FanoutPolicy{Concurrency: 0}}
	if fc := pol.FanoutConfig(); fc.Concurrency != 2 {
		t.Errorf("Concurrency(0) = %d, want 2 (floor)", fc.Concurrency)
	}
}
