package orchestrator

import (
	"testing"
	"time"
)

type stopIgnoringTimer struct{ stopped bool }

func (t *stopIgnoringTimer) Stop() bool {
	first := !t.stopped
	t.stopped = true
	return first
}

type stopIgnoringClock struct{ inner Clock }

func (c stopIgnoringClock) Now() time.Time { return c.inner.Now() }

func (c stopIgnoringClock) AfterFunc(d time.Duration, f func()) Timer {
	c.inner.AfterFunc(d, f)
	return &stopIgnoringTimer{}
}

func TestClockContract_StopBeforeTheDelayCatchesAClockWhoseStopDoesNotCancelTheRun(t *testing.T) {
	harnesses := []struct {
		name    string
		harness func() clockHarness
	}{
		{"fake", fakeHarness},
		{"system", systemHarness},
	}
	for _, c := range harnesses {
		t.Run(c.name, func(t *testing.T) {
			h := c.harness()
			h.clock = stopIgnoringClock{inner: h.clock}
			if err := stopBeforeTheDelayPreventsTheRun(h); err == nil {
				t.Fatal("the case passed on a clock whose Stop reports true but leaves the run armed: the stopped timer is too far out for the case to prove the contract")
			}
		})
	}
}
