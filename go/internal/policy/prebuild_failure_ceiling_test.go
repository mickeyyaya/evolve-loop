package policy

import "testing"

func TestPrebuildFailureCeiling_DefaultsToTwoAndAPositiveOverrideWins(t *testing.T) {
	t.Parallel()
	if got := DefaultSystemFailurePolicy().Thresholds.PrebuildFailureCeiling; got != 2 {
		t.Fatalf("compiled default = %d, want 2", got)
	}
	for override, want := range map[int]int{0: 2, 4: 4} {
		p := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{PrebuildFailureCeiling: override}}}
		cfg, err := p.FailurePolicyConfig()
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.Thresholds.PrebuildFailureCeiling; got != want {
			t.Errorf("override %d: ceiling = %d, want %d", override, got, want)
		}
	}
}
