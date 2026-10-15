package core

import (
	"testing"
	"time"
)

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
	executeRetryBackoff(0, 1)
	if *called {
		t.Errorf("nextAttempt < 2 must not sleep; slept %v", *slept)
	}

	executeRetryBackoff(1, 1)
	if !*called || *slept != 1*time.Second {
		t.Errorf("attempt 2: want exactly 1s sleep, got called=%v slept=%v", *called, *slept)
	}
}
