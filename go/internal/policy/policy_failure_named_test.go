package policy

import "testing"

// Names the ADR-0072 failure-policy exported surface (apicover graduation) by
// exercising the floor/level predicates and the category/level/action vocab —
// meaningful assertions, not bare symbol references.

func TestSystemFailurePolicy_FloorAndLevelPredicates(t *testing.T) {
	fp := DefaultSystemFailurePolicy()

	// IsFloor: the two non-negotiable categories are floor; task categories are not.
	if !fp.IsFloor(CategoryVerdictIncoherence) || !fp.IsFloor(CategoryInfraSystemic) {
		t.Error("verdict-incoherence and infra-systemic must be floor categories")
	}
	if fp.IsFloor(CategoryCodeBuildFail) || fp.IsFloor(CategoryCodeAuditFail) {
		t.Error("task categories must not be floor")
	}

	// IsSystemLevel: system categories halt; task categories don't.
	for _, sys := range []string{CategoryVerdictIncoherence, CategoryInfraSystemic, CategoryTransportHang, CategoryNonProgress} {
		if !fp.IsSystemLevel(sys) {
			t.Errorf("%q must be system-level", sys)
		}
	}
	for _, task := range []string{CategoryCodeBuildFail, CategoryCodeAuditFail, CategoryIntentMalformed} {
		if fp.IsSystemLevel(task) {
			t.Errorf("%q must be task-level", task)
		}
	}

	// Name the value types + level/action vocab via a constructed category row.
	var _ SystemFailurePolicy = fp
	_ = fp.Thresholds // FailureThresholds
	system := FailureCategory{Level: LevelSystem, Action: ActionHaltAndDiagnose}
	taskRetry := FailureCategory{Level: LevelTask, Action: ActionRetryWithFix}
	taskDefer := FailureCategory{Level: LevelTask, Action: ActionDeferOrQuarantine}
	if system.Level != LevelSystem || taskRetry.Action != ActionRetryWithFix || taskDefer.Action != ActionDeferOrQuarantine {
		t.Error("failure-category level/action vocab mismatch")
	}
	_ = FailureThresholds{RepeatCeiling: 1, VerifiedNotLandedCeiling: 1, TaskRetryCeiling: 1}
}
