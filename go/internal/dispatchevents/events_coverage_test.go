package dispatchevents

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestEmit_MarshalError covers the marshalFn error branch. json.Marshal
// of our well-typed Event struct can't fail in practice; the seam lets
// us prove the defensive error wrapping works.
func TestEmit_MarshalError(t *testing.T) {
	prev := marshalFn
	defer func() { marshalFn = prev }()
	marshalFn = func(any) ([]byte, error) { return nil, errors.New("synthetic marshal error") }

	w := NewWriter(t.TempDir())
	err := w.Emit(Event{EventType: EventClassification})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("error should mention marshal: %v", err)
	}
}

// TestEmit_WriteError covers the writeFn error branch (rare; would
// fire on disk-full or pipe-closed).
func TestEmit_WriteError(t *testing.T) {
	prev := writeFn
	defer func() { writeFn = prev }()
	writeFn = func(*os.File, []byte) (int, error) { return 0, errors.New("synthetic write error") }

	w := NewWriter(t.TempDir())
	err := w.Emit(Event{EventType: EventCounterNonAdvance})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Fatalf("error should mention write: %v", err)
	}
}
