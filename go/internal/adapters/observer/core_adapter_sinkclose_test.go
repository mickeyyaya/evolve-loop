package observer

import (
	"sync"
	"testing"
	"time"
)

type countingCloser struct {
	mu    sync.Mutex
	calls int
}

func (c *countingCloser) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return nil
}

func (c *countingCloser) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestCoreAdapter_SinkClosedOnNormalDone(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	close(done) // the watcher has already exited
	closer := &countingCloser{}

	start := time.Now()
	closeSinkAfterWait(done, 200*time.Millisecond, closer)
	elapsed := time.Since(start)

	if closer.Count() != 1 {
		t.Fatalf("normal-done path: Close() called %d times, want exactly 1", closer.Count())
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("normal-done path should return promptly once done fires; took %v", elapsed)
	}
}

func TestCoreAdapter_NoSinkCloseRaceOnTimeout(t *testing.T) {
	t.Parallel()
	done := make(chan struct{}) // never closed: a wedged watcher
	closer := &countingCloser{}

	timeout := 30 * time.Millisecond
	start := time.Now()
	closeSinkAfterWait(done, timeout, closer)
	elapsed := time.Since(start)

	if closer.Count() != 0 {
		t.Fatalf("timeout path: Close() called %d times, want 0 — closing a sink a still-running "+
			"goroutine may be writing to is the use-after-close race this fix must close", closer.Count())
	}
	if elapsed < timeout {
		t.Errorf("timeout path returned before the bound elapsed (%v < %v)", elapsed, timeout)
	}
}

func TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	close(done)
	closeSinkAfterWait(done, 200*time.Millisecond, nil)
}
