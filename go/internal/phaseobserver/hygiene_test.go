package phaseobserver

import (
	"testing"
	"time"
)

func TestStopChan_NilTimerNeverFires(t *testing.T) {
	if ch := stopChan(nil); ch != nil {
		t.Fatal("stopChan(nil) must return a nil channel (the never-fires idiom), got non-nil")
	}
	tm := time.NewTimer(time.Hour)
	defer tm.Stop()
	if ch := stopChan(tm); ch != tm.C {
		t.Fatal("stopChan(timer) must return the timer's own channel")
	}
}

func TestStopChan_ZeroAlloc(t *testing.T) {
	if n := testing.AllocsPerRun(100, func() { _ = stopChan(nil) }); n != 0 {
		t.Fatalf("stopChan(nil) must not allocate (was make(chan) per poll iteration), got %v allocs/op", n)
	}
}
