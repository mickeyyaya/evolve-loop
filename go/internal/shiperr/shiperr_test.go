package shiperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewShipError_Error(t *testing.T) {
	e := NewShipError(CodeGitPushRejected, ShipClassTransient, StageAtomicShip, "push lost the race")
	want := "[GIT_PUSH_REJECTED/transient @atomic-ship] push lost the race"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestNilError(t *testing.T) {
	var e *ShipError
	if got := e.Error(); got != "<nil ShipError>" {
		t.Errorf("nil Error() = %q", got)
	}
	if got := e.DebugString(); got != "" {
		t.Errorf("nil DebugString() = %q, want empty", got)
	}
}

func TestDebugString_Sorted(t *testing.T) {
	e := NewShipError(CodeSelfSHATampered, ShipClassIntegrity, StageVerifySelfSHA, "tamper",
		"expected", "aaa", "actual", "bbb")
	if got := e.DebugString(); got != "actual=bbb; expected=aaa" {
		t.Errorf("DebugString() = %q, want sorted %q", got, "actual=bbb; expected=aaa")
	}
}

func TestNewShipError_OddKV(t *testing.T) {
	e := NewShipError(CodeArgs, ShipClassConfig, StageArgs, "bad", "lonely")
	if v, ok := e.Debug["lonely"]; !ok || v != "" {
		t.Errorf("odd KV: Debug[lonely]=%q ok=%v, want empty+present", v, ok)
	}
}

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
