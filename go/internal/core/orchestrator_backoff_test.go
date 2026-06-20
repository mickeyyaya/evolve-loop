package core

import (
	"testing"
	"time"
)

// captureBackoff swaps in a recording sleep and returns pointers to the
// captured duration and a called-flag, plus a restore func. Tests the COMPUTED
// backoff duration directly via the seam — no wall-clock sleeping (instant) and
// exact (not a fuzzy time window). The called-flag is load-bearing for the
// zero-base case, where executeRetryBackoff returns before invoking the seam, so
// a 0 duration alone can't distinguish "not called" from "slept 0".
func captureBackoff() (*time.Duration, *bool, func()) {
	prev := backoffSleep
	var slept time.Duration
	var called bool
	backoffSleep = func(d time.Duration) { called = true; slept = d }
	return &slept, &called, func() { backoffSleep = prev }
}

func TestExecuteRetryBackoff_ZeroBaseDisables(t *testing.T) {
	_, called, restore := captureBackoff()
	defer restore()
	executeRetryBackoff(1, 0)
	if *called {
		t.Error("zero base must not invoke the sleep")
	}
}

func TestExecuteRetryBackoff_AppliedOnAttempt2(t *testing.T) {
	slept, called, restore := captureBackoff()
	defer restore()
	// nextAttempt = 1 (attempt = 0) -> below the >=2 threshold, no sleep.
	executeRetryBackoff(0, 1)
	if *called {
		t.Errorf("nextAttempt < 2 must not sleep; slept %v", *slept)
	}

	// nextAttempt = 2 (attempt = 1) -> base * 2^(2-2) = 1s exactly.
	executeRetryBackoff(1, 1)
	if !*called || *slept != 1*time.Second {
		t.Errorf("attempt 2: want exactly 1s sleep, got called=%v slept=%v", *called, *slept)
	}
}
