package shiperr

import (
	"slices"
	"testing"
)

func TestCodeGateWeakened_Vocab(t *testing.T) {
	if got, want := string(CodeGateWeakened), "GATE_WEAKENED"; got != want {
		t.Errorf("CodeGateWeakened = %q, want %q", got, want)
	}
	if SignalCode(CodeGateWeakened) != "SHIP_GATE_WEAKENED" {
		t.Errorf("SignalCode = %q, want SHIP_GATE_WEAKENED", SignalCode(CodeGateWeakened))
	}
	if !slices.Contains(AllCodes(), CodeGateWeakened) {
		t.Error("AllCodes does not list CodeGateWeakened, so its signal code is not registered")
	}
}
