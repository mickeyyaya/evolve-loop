package shiperr

import "testing"

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
