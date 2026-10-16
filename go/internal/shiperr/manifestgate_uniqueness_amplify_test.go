package shiperr

import (
	"strings"
	"testing"
)

func TestCodeManifestGate_DistinctFromEntireVocabulary(t *testing.T) {
	others := []ShipErrorCode{
		CodeSelfSHATampered, CodeSelfSHAIO,
		CodeAuditBindingHeadMoved, CodeAuditBindingTreeMismatch, CodeAuditBindingArtifactSHA,
		CodeAuditBindingArtifactMissing, CodeAuditBindingVerdictFail, CodeAuditBindingVerdictWarn,
		CodeAuditBindingMalformed, CodeAuditBindingDualVerdict, CodeAuditBindingStale,
		CodeAuditBindingNoAuditor, CodeAuditBindingAuditorExit, CodeAuditBindingNoLedger,
		CodeEGPSRedCount, CodeControlPlaneViolation, CodeInvalidClass,
		CodeManualNotTTY, CodeManualDeclined, CodeCommitGateMissing, CodeCommitGateStale,
		CodeCommitGateMalformed, CodeTrivialNotTrivial, CodeTrivialCriticalPaths,
		CodeGitDetachedHead, CodeGitStageFailed, CodeGitCommitFailed, CodeGitFFMergeDiverged,
		CodeGitFleetRebaseNeeded, CodeGitFleetRebaseConflict, CodeGitPushRejected,
		CodeCommitPrefixGate, CodeWorktreeResolve, CodeIntegrityTreeDrift,
		CodeArgs, CodeGitIO, CodeStateIO, CodeUnknown,
	}
	for _, c := range others {
		if c == CodeManifestGate {
			t.Errorf("CodeManifestGate aliases %q — must be a distinct constant", string(c))
		}
	}
}

func TestCodeManifestGate_WireStringNamingConvention(t *testing.T) {
	wire := string(CodeManifestGate)
	if wire != "MANIFEST_GATE" {
		t.Fatalf("CodeManifestGate wire string = %q, want %q", wire, "MANIFEST_GATE")
	}
	if strings.ToUpper(wire) != wire {
		t.Errorf("wire string %q is not all-uppercase", wire)
	}
	if strings.TrimSpace(wire) != wire {
		t.Errorf("wire string %q has surrounding whitespace", wire)
	}
}
