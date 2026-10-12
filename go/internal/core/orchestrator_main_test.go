package core

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

// TestMain zeroes the retry-backoff sleep for the whole core test suite, so
// the ~13 full-cycle retry/transient/backfill/timeout tests don't each pay
// the real ~5s backoff (~250s of wall-clock). Only the two backoff-unit tests
// mutate it afterward, and they're sequential (no t.Parallel); the Go runner
// drains sequential tests before launching parallel ones, so this stays
// race-safe.
func TestMain(m *testing.M) {
	backoffSleep = func(time.Duration) {}
	if err := gotooltest.LimitProcess(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := gittest.RemoveTemplates(); err != nil {
		fmt.Fprintln(os.Stderr, "core tests: remove git templates:", err)
	}
	os.Exit(code)
}
