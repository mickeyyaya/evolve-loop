package inboxfile

import (
	"errors"
	"testing"
)

func TestAPI_EveryExportIsNamed(t *testing.T) {
	if LockName != ".bump.lock" || LockWait <= 0 {
		t.Fatalf("lock constants drifted: %q %v", LockName, LockWait)
	}
	if !IsLifecycleOwned("status") || IsLifecycleOwned("id") {
		t.Fatal("IsLifecycleOwned must own status and not the id")
	}
	if ErrInvalidItem == nil || ErrAlreadyFiled == nil || errors.Is(ErrInvalidItem, ErrAlreadyFiled) {
		t.Fatal("the two sentinels must be distinct errors")
	}
}
