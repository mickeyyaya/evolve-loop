package shiperr

import (
	"errors"
	"fmt"
	"testing"
)

// TestNewShipError_Error pins the single-line render contract
// "[CODE/class @stage] message" — the ledger and debugger persona key off it.
func TestNewShipError_Error(t *testing.T) {
	e := NewShipError(CodeGitPushRejected, ShipClassTransient, StageAtomicShip, "push lost the race")
	want := "[GIT_PUSH_REJECTED/transient @atomic-ship] push lost the race"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestNilError guards the nil-receiver contract.
func TestNilError(t *testing.T) {
	var e *ShipError
	if got := e.Error(); got != "<nil ShipError>" {
		t.Errorf("nil Error() = %q", got)
	}
	if got := e.DebugString(); got != "" {
		t.Errorf("nil DebugString() = %q, want empty", got)
	}
}

// TestDebugString_Sorted pins deterministic (sorted-key) rendering.
func TestDebugString_Sorted(t *testing.T) {
	e := NewShipError(CodeSelfSHATampered, ShipClassIntegrity, StageVerifySelfSHA, "tamper",
		"expected", "aaa", "actual", "bbb")
	if got := e.DebugString(); got != "actual=bbb; expected=aaa" {
		t.Errorf("DebugString() = %q, want sorted %q", got, "actual=bbb; expected=aaa")
	}
}

// TestNewShipError_OddKV pins the "odd trailing key kept with empty value" rule
// — losing a diagnostic key is worse than recording a blank one.
func TestNewShipError_OddKV(t *testing.T) {
	e := NewShipError(CodeArgs, ShipClassConfig, StageArgs, "bad", "lonely")
	if v, ok := e.Debug["lonely"]; !ok || v != "" {
		t.Errorf("odd KV: Debug[lonely]=%q ok=%v, want empty+present", v, ok)
	}
}

// TestAsShipError recovers a *ShipError from anywhere in a wrapped chain.
func TestAsShipError(t *testing.T) {
	orig := NewShipError(CodeEGPSRedCount, ShipClassPrecondition, StageVerifyClass, "red")
	wrapped := fmt.Errorf("outer: %w", orig)
	got, ok := AsShipError(wrapped)
	if !ok || got.Code != CodeEGPSRedCount {
		t.Errorf("AsShipError = (%v,%v), want the wrapped ShipError", got, ok)
	}
	if _, ok := AsShipError(errors.New("plain")); ok {
		t.Errorf("AsShipError(plain) should be false")
	}
}
