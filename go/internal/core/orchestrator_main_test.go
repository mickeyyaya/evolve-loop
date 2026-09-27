package core

import (
	"os"
	"testing"
	"time"
)

// TestMain zeroes the retry-backoff sleep for the whole core test suite, so
// the ~13 full-cycle retry/transient/backfill/timeout tests don't each pay
// the real ~5s backoff (~250s of wall-clock). Only the two backoff-unit tests
// mutate it afterward, and they're sequential (no t.Parallel); the Go runner
// drains sequential tests before launching parallel ones, so this stays
// race-safe.
func TestMain(m *testing.M) {
	backoffSleep = func(time.Duration) {}
	os.Exit(m.Run())
}
