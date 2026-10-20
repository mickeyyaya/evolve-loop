package policy

import "testing"

func TestFailurePolicyConfig_TheExplanationCorrectionBudgetDefaultsToOneAndComesFromPolicy(t *testing.T) {
	t.Parallel()

	def, ok := DefaultSystemFailurePolicy().RetryPolicyFor(CategoryExplanationCorrection)
	if !ok || def.MaxRetries != 1 || def.Level != LevelTask || def.Action != ActionRetryWithFix {
		t.Fatalf("the compiled default is %+v (ok=%v), want one task-level retry-with-fix correction", def, ok)
	}
	got, err := Policy{SystemFailurePolicy: &SystemFailurePolicy{Categories: map[string]FailureCategory{
		CategoryExplanationCorrection: {Level: LevelTask, Action: ActionRetryWithFix, MaxRetries: 3},
	}}}.FailurePolicyConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := got.RetryPolicyFor(CategoryExplanationCorrection); c.MaxRetries != 3 {
		t.Fatalf("policy.json did not set the budget: %+v", c)
	}
}
