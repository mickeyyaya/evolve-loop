package shiperr

import "testing"

// TestCodeRepoContractInfra_Vocab pins the cycle-1409 infra-classification
// code's wire value and — the load-bearing half — its NON-ALIASING of
// CodeRepoContractGate. The whole point of the split is that a router or
// dashboard can distinguish "a repo-wide guard suite is genuinely RED in this
// lane" (fix your code) from "the scanner pack died twice with no test-level
// failure" (infra fault, safe to re-dispatch). If these two ever collapse onto
// one value, the cycle-1402/1403/1405 false-RED storm is back with no signal.
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
