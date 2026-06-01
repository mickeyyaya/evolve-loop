package fixtures

import (
	"sync/atomic"
	"time"
)

// FixedClock returns a deterministic clock function that yields start on the
// first call and advances by exactly one step on EVERY subsequent call
// (start, start+step, start+2·step, …). It replaces the copy-pasted
// fixedClock(t, dur) helpers the phase/runner tests used to assert
// deterministic DurationMS without touching the wall clock.
//
// Contract note: the helpers this replaces returned start, then start+dur for
// ALL later calls (a two-position toggle). FixedClock strictly advances —
// identical for the 2-call (start/end) pattern the runner uses, but it diverges
// on a 3rd call, so prefer a fresh clock per timed unit. The returned closure
// is goroutine-safe (atomic counter).
//
//	clock := fixtures.FixedClock(time.Unix(1_700_000_000, 0), 200*time.Millisecond)
//	clock() // start
//	clock() // start + 200ms
//	clock() // start + 400ms
func FixedClock(start time.Time, step time.Duration) func() time.Time {
	var calls int64
	return func() time.Time {
		n := atomic.AddInt64(&calls, 1) - 1
		return start.Add(time.Duration(n) * step)
	}
}
