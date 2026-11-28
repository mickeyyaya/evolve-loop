package cycleidentity

import "testing"

func TestAPINames(t *testing.T) {
	t.Parallel()
	for got, want := range map[Part]string{PartGraph: "graph", PartContract: "contract", PartPolicy: "policy"} {
		if string(got) != want {
			t.Errorf("%q != %q", got, want)
		}
	}
	var _ Identity
	var _ Sources
	if ErrNoIdentity == nil || ErrMismatch == nil {
		t.Fatal("sentinels missing")
	}
	mismatch := &MismatchError{Parts: []Part{PartGraph}}
	if mismatch.Unwrap() != ErrMismatch || len(mismatch.Error()) == 0 {
		t.Errorf("MismatchError = %q, unwrap %v", mismatch.Error(), mismatch.Unwrap())
	}
}
