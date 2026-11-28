package orchestrator

import (
	"testing"
	"time"
)

const stallWindow = 100 * time.Millisecond

type stallingClock struct{ inner Clock }

func (c stallingClock) Now() time.Time { return c.inner.Now() }

func (c stallingClock) AfterFunc(d time.Duration, f func()) Timer {
	fired := make(chan struct{})
	timer := c.inner.AfterFunc(d, func() { close(fired); f() })
	select {
	case <-fired:
	case <-time.After(stallWindow):
	}
	return timer
}

func TestClockContract_StopBeforeTheDelayHoldsWhenTheCallerIsDescheduled(t *testing.T) {
	h := systemHarness()
	h.clock = stallingClock{inner: h.clock}
	if err := stopBeforeTheDelayPreventsTheRun(h); err != nil {
		t.Fatal(err)
	}
}
