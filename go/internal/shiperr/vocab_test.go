package shiperr

import "testing"

// TestVocabularyWireStrings pins the EXACT wire string of every Code/Class/Stage
// constant. These strings are stable contract: ledger entries, the debugger
// persona, and downstream tests key off them verbatim, so a drift is a
// trust-kernel break. Slice-of-struct (not map) so the constant IDENTIFIER is
// asserted against its literal — a typo'd const value is caught.
func TestVocabularyWireStrings(t *testing.T) {
	codes := []struct {
		got  ShipErrorCode
		want string
	}{
		{CodeSelfSHATampered, "SELF_SHA_TAMPERED"},
		{CodeSelfSHAIO, "SELF_SHA_IO"},
		{CodeAuditBindingHeadMoved, "AUDIT_BINDING_HEAD_MOVED"},
		{CodeAuditBindingTreeMismatch, "AUDIT_BINDING_TREE_MISMATCH"},
		{CodeAuditBindingArtifactSHA, "AUDIT_BINDING_ARTIFACT_SHA"},
		{CodeAuditBindingArtifactMissing, "AUDIT_BINDING_ARTIFACT_MISSING"},
		{CodeAuditBindingVerdictFail, "AUDIT_BINDING_VERDICT_FAIL"},
		{CodeAuditBindingVerdictWarn, "AUDIT_BINDING_VERDICT_WARN_STRICT"},
		{CodeAuditBindingMalformed, "AUDIT_BINDING_MALFORMED_VERDICT"},
		{CodeAuditBindingDualVerdict, "AUDIT_BINDING_DUAL_VERDICT"},
		{CodeAuditBindingStale, "AUDIT_BINDING_STALE"},
		{CodeAuditBindingNoAuditor, "AUDIT_BINDING_NO_AUDITOR"},
		{CodeAuditBindingAuditorExit, "AUDIT_BINDING_AUDITOR_EXIT"},
		{CodeAuditBindingNoLedger, "AUDIT_BINDING_NO_LEDGER"},
		{CodeEGPSRedCount, "EGPS_RED_COUNT"},
		{CodeControlPlaneViolation, "CONTROL_PLANE_VIOLATION"},
		{CodeInvalidClass, "INVALID_CLASS"},
		{CodeManualNotTTY, "MANUAL_NOT_TTY"},
		{CodeManualDeclined, "MANUAL_DECLINED"},
		{CodeCommitGateMissing, "COMMIT_GATE_MISSING"},
		{CodeCommitGateStale, "COMMIT_GATE_STALE"},
		{CodeCommitGateMalformed, "COMMIT_GATE_MALFORMED"},
		{CodeTrivialNotTrivial, "TRIVIAL_NOT_TRIVIAL"},
		{CodeTrivialCriticalPaths, "TRIVIAL_CRITICAL_PATHS"},
		{CodeGitDetachedHead, "GIT_DETACHED_HEAD"},
		{CodeGitStageFailed, "GIT_STAGE_FAILED"},
		{CodeGitCommitFailed, "GIT_COMMIT_FAILED"},
		{CodeGitFFMergeDiverged, "GIT_FF_MERGE_DIVERGED"},
		{CodeGitFleetRebaseNeeded, "GIT_FLEET_REBASE_NEEDED"},
		{CodeGitFleetRebaseConflict, "GIT_FLEET_REBASE_CONFLICT"},
		{CodeGitPushRejected, "GIT_PUSH_REJECTED"},
		{CodeCommitPrefixGate, "COMMIT_PREFIX_GATE"},
		{CodeManifestGate, "MANIFEST_GATE"},
		{CodeWorktreeResolve, "WORKTREE_RESOLVE"},
		{CodeIntegrityTreeDrift, "INTEGRITY_TREE_DRIFT"},
		{CodeArgs, "ARGS"},
		{CodeGitIO, "GIT_IO"},
		{CodeStateIO, "STATE_IO"},
		{CodeUnknown, "UNKNOWN"},
	}
	for _, c := range codes {
		if string(c.got) != c.want {
			t.Errorf("code = %q, want %q", c.got, c.want)
		}
	}

	classes := []struct {
		got  ShipErrorClass
		want string
	}{
		{ShipClassTransient, "transient"},
		{ShipClassPrecondition, "precondition"},
		{ShipClassIntegrity, "integrity"},
		{ShipClassConfig, "config"},
	}
	for _, c := range classes {
		if string(c.got) != c.want {
			t.Errorf("class = %q, want %q", c.got, c.want)
		}
	}

	stages := []struct {
		got  ShipStage
		want string
	}{
		{StageVerifySelfSHA, "verify-self-sha"},
		{StageVerifyClass, "verify-class"},
		{StageAtomicShip, "atomic-ship"},
		{StagePostShip, "post-ship"},
		{StageArgs, "args"},
	}
	for _, s := range stages {
		if string(s.got) != s.want {
			t.Errorf("stage = %q, want %q", s.got, s.want)
		}
	}
}
