package shiperr

import "testing"

func TestCodeRepoContractInfra_Vocab(t *testing.T) {
	if got, want := string(CodeRepoContractInfra), "REPO_CONTRACT_INFRA"; got != want {
		t.Errorf("CodeRepoContractInfra = %q, want %q", got, want)
	}
	if CodeRepoContractInfra == CodeRepoContractGate {
		t.Error("CodeRepoContractInfra must NOT alias CodeRepoContractGate — the split IS the fix")
	}
	if CodeRepoContractInfra == CodeGitStageFailed || CodeRepoContractInfra == CodeManifestGate || CodeRepoContractInfra == CodeUnknown {
		t.Error("CodeRepoContractInfra must not alias git-stage, manifest-gate, or the unknown fallthrough")
	}
	err := NewShipError(CodeRepoContractInfra, ShipClassPrecondition, StageAtomicShip, "pack died twice, no test failure")
	if err.Code != CodeRepoContractInfra || err.Class != ShipClassPrecondition || err.Stage != StageAtomicShip {
		t.Fatalf("constructed ShipError = %+v", err)
	}
	if got, ok := AsShipError(err); !ok || got.Code != CodeRepoContractInfra {
		t.Fatalf("AsShipError must recover the infra code, got %+v ok=%v", got, ok)
	}
}
