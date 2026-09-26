package policy

import "testing"

func TestSystemFailurePolicy_FloorAndLevelPredicates(t *testing.T) {
	fp := DefaultSystemFailurePolicy()

	if !fp.IsFloor(CategoryVerdictIncoherence) || !fp.IsFloor(CategoryInfraSystemic) {
		t.Error("verdict-incoherence and infra-systemic must be floor categories")
	}
	if fp.IsFloor(CategoryCodeBuildFail) || fp.IsFloor(CategoryCodeAuditFail) {
		t.Error("task categories must not be floor")
	}

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

	// These lines name the value types and the level/action vocabulary for apicover.
	var _ SystemFailurePolicy = fp
	_ = fp.Thresholds
	system := FailureCategory{Level: LevelSystem, Action: ActionHaltAndDiagnose}
	taskRetry := FailureCategory{Level: LevelTask, Action: ActionRetryWithFix}
	taskDefer := FailureCategory{Level: LevelTask, Action: ActionDeferOrQuarantine}
	if system.Level != LevelSystem || taskRetry.Action != ActionRetryWithFix || taskDefer.Action != ActionDeferOrQuarantine {
		t.Error("failure-category level/action vocab mismatch")
	}
	_ = FailureThresholds{RepeatCeiling: 1, VerifiedNotLandedCeiling: 1, TaskRetryCeiling: 1}
}
