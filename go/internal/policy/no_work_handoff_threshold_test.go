package policy

import "testing"

func TestFailurePolicyConfig_TheNoWorkHandoffCeilingDefaultsToSixAndComesFromPolicy(t *testing.T) {
	t.Parallel()

	if got := DefaultSystemFailurePolicy().Thresholds.NoWorkHandoffHaltCeiling; got != 6 {
		t.Fatalf("compiled default = %d, want 6", got)
	}
	got, err := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{NoWorkHandoffHaltCeiling: 9}}}.FailurePolicyConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Thresholds.NoWorkHandoffHaltCeiling != 9 {
		t.Fatalf("policy.json did not set the ceiling: %d", got.Thresholds.NoWorkHandoffHaltCeiling)
	}
	kept, _ := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{NoWorkHandoffHaltCeiling: -1}}}.FailurePolicyConfig()
	if kept.Thresholds.NoWorkHandoffHaltCeiling != 6 {
		t.Fatalf("a non-positive value keeps the default: %d", kept.Thresholds.NoWorkHandoffHaltCeiling)
	}
}
