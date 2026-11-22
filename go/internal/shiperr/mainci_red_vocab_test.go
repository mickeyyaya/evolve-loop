package shiperr

import "testing"

func TestCodeMainCIRed_Vocab(t *testing.T) {
	if got, want := string(CodeMainCIRed), "MAIN_CI_RED"; got != want {
		t.Errorf("CodeMainCIRed = %q, want %q", got, want)
	}
	if CodeMainCIRed == CodeMainRedPark || CodeMainCIRed == CodeRepoContractGate || CodeMainCIRed == CodeGitFleetRebaseNeeded || CodeMainCIRed == CodeUnknown {
		t.Error("CodeMainCIRed must not alias the lane park, the lane-blamed code, the rebase code or the unknown fallthrough")
	}
	err := NewShipError(CodeMainCIRed, ShipClassPrecondition, StageAtomicShip, "origin/main CI is red")
	if got, ok := AsShipError(err); !ok || got.Code != CodeMainCIRed || got.Class != ShipClassPrecondition {
		t.Fatalf("AsShipError must recover the CI code, got %+v ok=%v", got, ok)
	}
}
