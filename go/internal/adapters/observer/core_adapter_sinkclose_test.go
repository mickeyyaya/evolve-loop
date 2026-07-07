package observer

// core_adapter_sinkclose_test.go — RED contract for the fable5 deep-scan
// finding observer-sink-close-race (inbox weight 0.92, cycle-618 scout).
//
// Context. Start's returned cancel closure (core_adapter.go) waits for the
// watcher goroutine with a bounded 10s timeout, then UNCONDITIONALLY closes
// the events sink regardless of which select arm fired:
//
//	select {
//	case <-done:
//	case <-time.After(10 * time.Second):
//	    fmt.Fprintf(os.Stderr, "... leaking goroutine ...")
//	}
//	if sinkCloser != nil {
//	    _ = sinkCloser.Close()
//	}
//
// When the watcher goroutine is genuinely wedged (e.g. a hung liveness probe
// or a stuck sink write) past the 10s bound, the timeout arm fires but the
// leaked goroutine is still running and may still be mid-write to the sink.
// Closing the sink out from under it is a use-after-close race — the leaked
// goroutine's next Write can return an error, or (on some platforms/sink
// implementations) corrupt concurrent state, and the returned error is
// swallowed (`_ = sinkCloser.Close()` / the watcher discards its Watch
// error), so the race is invisible until it manifests as a flake.
//
// The fix extracts the wait+close decision into an isolated, directly
// testable primitive:
//
//	func closeSinkAfterWait(done <-chan struct{}, timeout time.Duration, closer io.Closer)
//
// which closes `closer` ONLY when `done` fires within `timeout` — never on
// the timeout arm, so a still-running leaked goroutine's sink is never
// closed under it. Start's cancel closure delegates to this helper with the
// real done channel, sinkCloser, and the production 10s bound.
//
// RED today: closeSinkAfterWait is undefined, so this file fails to compile
// — the intended RED signal before Builder implements the helper and wires
// Start's cancel closure to call it instead of the inline unconditional
// Close.

import (
	"sync"
	"testing"
	"time"
)

// countingCloser is a goroutine-safe io.Closer that records how many times
// Close was called, so a test can assert on close COUNT rather than just
// absence-of-panic (the cheapest gaming fake — a closer that does nothing
// observable — would pass a weaker assertion regardless of the fix).
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

// TestCoreAdapter_SinkClosedOnNormalDone is the positive/regression twin: when
// the watcher goroutine finishes within the bound (the overwhelmingly common
// case — a healthy phase completing), the sink MUST still be closed exactly
// once so file descriptors/handles do not leak on the normal path. This twin
// stops a degenerate "never close" fix (which would trivially satisfy the
// timeout test below) from passing.
func TestCoreAdapter_SinkClosedOnNormalDone(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	close(done) // watcher already finished before the wait begins
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

// TestCoreAdapter_NoSinkCloseRaceOnTimeout is the negative test proving the
// race is closed: when the watcher goroutine is still running past the bound
// (done never fires — the genuinely-wedged/leaked-goroutine scenario the 10s
// comment describes), the sink must NEVER be closed underneath it. A
// no-op-that-always-closes implementation (the pre-fix behavior) fails this
// assertion.
func TestCoreAdapter_NoSinkCloseRaceOnTimeout(t *testing.T) {
	t.Parallel()
	done := make(chan struct{}) // never closed — simulates a wedged watcher
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

// TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe guards the existing
// `if sinkCloser != nil` contract Start relies on (a.Sink caller-supplied
// writer with no Closer capability) — the extracted helper must preserve it
// rather than panicking on a nil closer.
func TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	close(done)
	closeSinkAfterWait(done, 200*time.Millisecond, nil) // must not panic
}
