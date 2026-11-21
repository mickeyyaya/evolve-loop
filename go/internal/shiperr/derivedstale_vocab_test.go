package shiperr

import (
	"slices"
	"testing"
)

func TestCodeDerivedStale_Vocab(t *testing.T) {
	if got, want := string(CodeDerivedStale), "DERIVED_STALE"; got != want {
		t.Errorf("CodeDerivedStale = %q, want %q", got, want)
	}
	if SignalCode(CodeDerivedStale) != "SHIP_DERIVED_STALE" {
		t.Errorf("SignalCode = %q, want SHIP_DERIVED_STALE", SignalCode(CodeDerivedStale))
	}
	if !slices.Contains(AllCodes(), CodeDerivedStale) {
		t.Error("AllCodes does not list CodeDerivedStale, so its signal code is not registered")
	}
}
