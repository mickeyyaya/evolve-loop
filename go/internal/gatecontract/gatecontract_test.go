package gatecontract

import (
	"slices"
	"testing"
)

func TestTrustClasses_AreTheFiveWordsOfTheInventory(t *testing.T) {
	t.Parallel()
	want := []string{"authority", "verified", "subject", "advisory", "open-seam"}
	if got := TrustClasses(); !slices.Equal(got, want) {
		t.Errorf("TrustClasses() = %v, want %v", got, want)
	}
}

func TestKinds_AreScannerPhaseFloorAndShipCheck(t *testing.T) {
	t.Parallel()
	want := []string{"scanner", "phase-floor", "ship-check"}
	if got := Kinds(); !slices.Equal(got, want) {
		t.Errorf("Kinds() = %v, want %v", got, want)
	}
}

func TestStages_AreShadowAndEnforce(t *testing.T) {
	t.Parallel()
	want := []string{"shadow", "enforce"}
	if got := Stages(); !slices.Equal(got, want) {
		t.Errorf("Stages() = %v, want %v", got, want)
	}
}

func TestLists_ReturnACopyThatACallerCannotMutate(t *testing.T) {
	t.Parallel()
	for name, list := range map[string]func() []string{"TrustClasses": TrustClasses, "Kinds": Kinds, "Stages": Stages} {
		first := list()
		first[0] = "changed"
		if list()[0] == "changed" {
			t.Errorf("%s() shares its backing array with the caller", name)
		}
	}
}

func TestClassConstants_SpellTheWordsOfTheInventory(t *testing.T) {
	t.Parallel()
	for got, want := range map[string]string{ClassAuthority: "authority", ClassVerified: "verified", ClassSubject: "subject", ClassAdvisory: "advisory", ClassOpenSeam: "open-seam"} {
		if got != want || !slices.Contains(TrustClasses(), got) {
			t.Errorf("class constant %q, want %q and a member of TrustClasses()", got, want)
		}
	}
}
