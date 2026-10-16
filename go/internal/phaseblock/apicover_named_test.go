package phaseblock

import (
	"errors"
	"testing"
)

func TestNamePublicAPI(t *testing.T) {
	var (
		_ DigestSource = fakeSource{}
		_ Provenance   = allOK
		_ Digest
		_ = ErrEmptyChain
	)

	d, err := Compute("scout", "run", "ts", "", fakeSource{bin: "b", commit: "c"})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if err := Verify([]Digest{d}, "b", "c", allOK); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	te := &TamperError{Phase: "p", Reason: "r"}
	if te.Error() == "" {
		t.Fatal("TamperError.Error() must be non-empty")
	}
	if !errors.Is(Verify(nil, "b", "c", allOK), ErrEmptyChain) {
		t.Fatal("empty chain must surface ErrEmptyChain")
	}
}
