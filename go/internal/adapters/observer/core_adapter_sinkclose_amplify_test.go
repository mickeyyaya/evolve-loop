package observer

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// amplFakeCloser records Close() calls/errors independently of any fixture
// the existing RED suite may already define, so this file adds coverage
// without depending on it.
type amplFakeCloser struct {
	mu       sync.Mutex
	closed   int
	closeErr error
}

func (f *amplFakeCloser) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return f.closeErr
}

func (f *amplFakeCloser) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

func TestCloseSinkAfterWait_ReturnValueTrueOnDone(t *testing.T) {
	done := make(chan struct{})
	close(done)
	fc := &amplFakeCloser{}
	got := closeSinkAfterWait(done, time.Hour, fc)
	if !got {
		t.Fatalf("expected true when done fires before timeout, got false")
	}
	if fc.count() != 1 {
		t.Fatalf("expected Close() called exactly once, got %d", fc.count())
	}
}

func TestCloseSinkAfterWait_ReturnValueFalseOnTimeout(t *testing.T) {
	done := make(chan struct{}) // never closed
	fc := &amplFakeCloser{}
	got := closeSinkAfterWait(done, 5*time.Millisecond, fc)
	if got {
		t.Fatalf("expected false on the timeout arm, got true")
	}
	if fc.count() != 0 {
		t.Fatalf("expected Close() NOT called on the timeout arm, got %d calls", fc.count())
	}
}

func TestCloseSinkAfterWait_CloserErrorDoesNotPanic(t *testing.T) {
	done := make(chan struct{})
	close(done)
	fc := &amplFakeCloser{closeErr: errors.New("boom: disk full")}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("closeSinkAfterWait panicked on a Close() error: %v", r)
		}
	}()
	got := closeSinkAfterWait(done, time.Second, fc)
	if !got {
		t.Fatalf("expected true (done fired) even though Close() returned an error")
	}
	if fc.count() != 1 {
		t.Fatalf("expected Close() called once despite it returning an error, got %d", fc.count())
	}
}

func TestCloseSinkAfterWait_NilDoneChannelAlwaysTimesOut(t *testing.T) {
	var done chan struct{} // nil: blocks forever in select, must fall through to the timeout arm
	fc := &amplFakeCloser{}
	got := closeSinkAfterWait(done, 10*time.Millisecond, fc)
	if got {
		t.Fatalf("expected false: a nil done channel can never fire")
	}
	if fc.count() != 0 {
		t.Fatalf("expected Close() NOT called when done is nil, got %d calls", fc.count())
	}
}

// TestCloseSinkAfterWait_ConcurrentInvocations_NoRaceOnDistinctClosers exercises
// the exact production shape the fix targets: many phases' Start() cancel
// closures racing to wait+close concurrently in the batch dispatch path. Each
// goroutine owns an independent done/closer pair; run with -race to catch any
// shared mutable state closeSinkAfterWait might introduce.
func TestCloseSinkAfterWait_ConcurrentInvocations_NoRaceOnDistinctClosers(t *testing.T) {
	const n = 50
	var wg sync.WaitGroup
	var trueCount int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			done := make(chan struct{})
			fc := &amplFakeCloser{}
			if i%2 == 0 {
				close(done) // half fire immediately
			}
			if closeSinkAfterWait(done, 20*time.Millisecond, fc) {
				atomic.AddInt64(&trueCount, 1)
				if fc.count() != 1 {
					t.Errorf("goroutine %d: expected Close() called once on the done-fired path, got %d", i, fc.count())
				}
			} else if fc.count() != 0 {
				t.Errorf("goroutine %d: expected Close() NOT called on the timeout path, got %d", i, fc.count())
			}
		}(i)
	}
	wg.Wait()
	if trueCount != n/2 {
		t.Fatalf("expected exactly %d done-fired invocations, got %d", n/2, trueCount)
	}
}
