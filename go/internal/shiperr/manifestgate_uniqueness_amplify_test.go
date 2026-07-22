package shiperr

import (
	"strings"
	"testing"
)

// TestCodeManifestGate_DistinctFromEntireVocabulary broadens the existing
// distinctness check (which only pins CodeManifestGate != CodeGitStageFailed,
// the one code it replaced) to the FULL wire vocabulary. The ledger and
// debugger persona key routing off the wire string; a collision with ANY
// other code — not just the one it replaced — would silently misroute.
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

// TestCodeManifestGate_WireStringNamingConvention pins the wire string's
// exact byte content: uppercase, underscore-separated, no surrounding
// whitespace — the convention every sibling code in vocab_test.go follows. A
// future refactor that accidentally lowercases or pads the constant would
// break ledger string-matching silently.
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
