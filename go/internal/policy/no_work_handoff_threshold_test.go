package policy

import "testing"

func TestFailurePolicyConfig_TheNoWorkHandoffCeilingAndWindowHaveDefaultsAndComeFromPolicy(t *testing.T) {
	t.Parallel()

	def := DefaultSystemFailurePolicy().Thresholds
	if def.NoWorkHandoffHaltCeiling != 4 || def.NoWorkHandoffWindow != 12 {
		t.Fatalf("compiled defaults = ceiling %d window %d, want 4 and 12", def.NoWorkHandoffHaltCeiling, def.NoWorkHandoffWindow)
	}
	got, err := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{NoWorkHandoffHaltCeiling: 9, NoWorkHandoffWindow: 20}}}.FailurePolicyConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Thresholds.NoWorkHandoffHaltCeiling != 9 || got.Thresholds.NoWorkHandoffWindow != 20 {
		t.Fatalf("policy.json did not set them: %+v", got.Thresholds)
	}
	kept, _ := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{NoWorkHandoffHaltCeiling: -1, NoWorkHandoffWindow: -1}}}.FailurePolicyConfig()
	if kept.Thresholds.NoWorkHandoffHaltCeiling != 4 || kept.Thresholds.NoWorkHandoffWindow != 12 {
		t.Fatalf("a non-positive value keeps the default: %+v", kept.Thresholds)
	}
}
