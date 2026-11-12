package shiperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodeMainRedPark_Vocab(t *testing.T) {
	if got, want := string(CodeMainRedPark), "MAIN_RED_PARK"; got != want {
		t.Errorf("CodeMainRedPark = %q, want %q", got, want)
	}
	if CodeMainRedPark == CodeRepoContractGate || CodeMainRedPark == CodeGitFleetRebaseNeeded || CodeMainRedPark == CodeUnknown {
		t.Error("CodeMainRedPark must not alias the lane-blamed code, the rebase code or the unknown fallthrough")
	}
	err := NewShipError(CodeMainRedPark, ShipClassPrecondition, StageAtomicShip, "red on the base and on main")
	if got, ok := AsShipError(err); !ok || got.Code != CodeMainRedPark || got.Class != ShipClassPrecondition {
		t.Fatalf("AsShipError must recover the park code, got %+v ok=%v", got, ok)
	}
}

func TestShipError_OnlyTheParkCodeIsErrMainRedParked(t *testing.T) {
	park := NewShipError(CodeMainRedPark, ShipClassPrecondition, StageAtomicShip, "red on the base and on main")
	if !errors.Is(fmt.Errorf("phase ship: %w", park), ErrMainRedParked) {
		t.Error("a wrapped park error is not ErrMainRedParked")
	}
	blamed := NewShipError(CodeRepoContractGate, ShipClassIntegrity, StageAtomicShip, "red on the lane")
	var none *ShipError
	if errors.Is(blamed, ErrMainRedParked) || errors.Is(park, errors.New("other")) || none.Is(ErrMainRedParked) {
		t.Error("a lane-blamed error, another target or a nil ship error reads as a park")
	}
}
