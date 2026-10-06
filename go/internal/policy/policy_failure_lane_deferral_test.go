package policy

import "testing"

func TestLaneDeferralHaltCeiling_DefaultsToThreeAndMergesFromOperatorPolicy(t *testing.T) {
	if got := DefaultSystemFailurePolicy().Thresholds.LaneDeferralHaltCeiling; got != 3 {
		t.Fatalf("compiled default LaneDeferralHaltCeiling = %d, want 3", got)
	}
	p := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{LaneDeferralHaltCeiling: 5}}}
	cfg, err := p.FailurePolicyConfig()
	if err != nil {
		t.Fatalf("FailurePolicyConfig: %v", err)
	}
	if cfg.Thresholds.LaneDeferralHaltCeiling != 5 || cfg.Thresholds.ConsecutiveFailuresHaltCeiling != 3 {
		t.Errorf("override = %d (want 5), sibling = %d (want the default 3)", cfg.Thresholds.LaneDeferralHaltCeiling, cfg.Thresholds.ConsecutiveFailuresHaltCeiling)
	}
}
