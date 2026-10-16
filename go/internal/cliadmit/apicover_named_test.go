package cliadmit

import (
	"context"
	"testing"
	"time"
)

func TestApicoverNamed_DefaultTTL(t *testing.T) {
	if DefaultTTL <= 0 {
		t.Fatal("DefaultTTL must be positive")
	}
}

func TestApicoverNamed_Acquire(t *testing.T) {
	setSlotDir(t)
	var release func()
	var err error
	release, err = Acquire(context.Background(), "apicover-cli", 0, DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	_ = time.Second
}
