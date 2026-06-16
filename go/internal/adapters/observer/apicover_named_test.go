package observer

import (
	"bytes"
	"testing"
	"time"
)

// TestObserver_NewReturnsTypedHandleWithStopContract names the observer.Observer
// type (New returns *Observer but the bare type is never named in a test) and
// pins its lifecycle contract: New returns a usable *Observer and Stop is
// idempotent (the once-guard at observer.go:222 never double-closes quit).
func TestObserver_NewReturnsTypedHandleWithStopContract(t *testing.T) {
	t.Parallel()
	var o *Observer = New(Config{StallS: time.Hour, PollS: 10 * time.Millisecond}, &bytes.Buffer{})
	if o == nil {
		t.Fatal("New returned a nil *Observer")
	}
	if err := o.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := o.Stop(); err != nil {
		t.Fatalf("second Stop must be idempotent (sync.Once guard), got %v", err)
	}
}
