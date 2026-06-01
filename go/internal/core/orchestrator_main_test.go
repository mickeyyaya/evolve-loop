package core

import (
	"os"
	"testing"
	"time"
)

// TestMain zeroes the retry-backoff sleep for the whole core test suite. The
// real backoff (default base 5s) is a production behavior, but ~13 full-cycle
// retry/transient/backfill/timeout tests would each sleep it for real — ~250s
// of pure wall-clock. Stubbing the single sleep seam here makes them all fast
// (and any future retry test automatically), with no per-test env churn.
//
// Set BEFORE m.Run() and mutated afterward only by the two backoff-unit tests,
// which are sequential (no t.Parallel) and save/restore. The Go runner drains
// sequential tests before launching parallel ones, so any test that READS the
// seam concurrently never overlaps those writes → race-safe (verified -race).
func TestMain(m *testing.M) {
	backoffSleep = func(time.Duration) {}
	os.Exit(m.Run())
}
