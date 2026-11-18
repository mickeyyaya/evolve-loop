package phaseblock

import (
	"errors"
	"testing"
)

func TestNamePublicAPI(t *testing.T) {
	var src DigestSource = fakeSource{bin: "b", commit: "c"}
	var d Digest
	d, err := Compute("scout", "run", "ts", "", src)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	var prov Provenance = allOK
	if err := Verify([]Digest{d}, "b", "c", prov); err != nil {
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
