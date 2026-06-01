package fixtures

import "time"

// FixedClock returns a deterministic clock function that yields start on the
// first call and advances by step on every subsequent call. It replaces the
// copy-pasted fixedClock(t, dur) helpers used by the phase/runner tests to
// assert deterministic DurationMS without touching the wall clock.
//
//	clock := fixtures.FixedClock(time.Unix(1_700_000_000, 0), 200*time.Millisecond)
//	clock() // start
//	clock() // start + 200ms
//	clock() // start + 400ms
func FixedClock(start time.Time, step time.Duration) func() time.Time {
	calls := 0
	return func() time.Time {
		t := start.Add(time.Duration(calls) * step)
		calls++
		return t
	}
}
