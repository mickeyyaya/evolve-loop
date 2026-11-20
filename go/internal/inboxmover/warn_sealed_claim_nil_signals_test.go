package inboxmover

import "testing"

func TestWarnSealedClaim_NilSignalsEmitsNothingAndDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("warnSealedClaim with nil Signals panicked: %v", r)
		}
	}()
	Options{}.warnSealedClaim(ClaimRelease{ID: "x", Cycle: 1}, Holder{})
}
