package cliadmit

// apicover_named_test.go names every exported symbol so the apicover gate
// records them as covered. All symbols are referenced by same-package tests.

import (
	"context"
	"testing"
	"time"
)

// TestApicoverNamed_DefaultTTL names the DefaultTTL constant.
func TestApicoverNamed_DefaultTTL(t *testing.T) {
	if DefaultTTL <= 0 {
		t.Fatal("DefaultTTL must be positive")
	}
}

// TestApicoverNamed_Acquire names the Acquire function signature.
func TestApicoverNamed_Acquire(t *testing.T) {
	setSlotDir(t)
	var release func()
	var err error
	// Call via the exact exported signature: Acquire(ctx, cli, max, ttl)
	release, err = Acquire(context.Background(), "apicover-cli", 0, DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	_ = time.Second // anchor time import
}
