package shiperr

import "testing"

// TestCodeRepoContractGate_Vocab pins the dedicated repo-contract gate code's
// wire value and non-aliasing (mirrors manifestgate_vocab_test.go): the
// ledger/debugger must be able to tell "a repo-wide guard suite is RED in
// this lane" from any git-level failure, and a contract block inherits the
// retry-friendly precondition class.
func TestCodeRepoContractGate_Vocab(t *testing.T) {
	if got, want := string(CodeRepoContractGate), "REPO_CONTRACT_GATE"; got != want {
		t.Errorf("CodeRepoContractGate = %q, want %q", got, want)
	}
	if CodeRepoContractGate == CodeGitStageFailed || CodeRepoContractGate == CodeManifestGate {
		t.Error("CodeRepoContractGate must not alias git-stage or manifest-gate codes")
	}
	err := NewShipError(CodeRepoContractGate, ShipClassPrecondition, StageAtomicShip, "scanner pack RED")
	if err.Code != CodeRepoContractGate || err.Class != ShipClassPrecondition {
		t.Fatalf("constructed ShipError = %+v", err)
	}
}
