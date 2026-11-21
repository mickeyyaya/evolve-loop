package shipwindowtest

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

func moveClock(c *Clock, d time.Duration) { c.Advance(d) }

func TestClock_AdvanceMovesNowByTheStep(t *testing.T) {
	c := NewClock()
	start := c.Now()
	moveClock(c, 20*time.Minute)
	if got := c.Now().Sub(start); got != 20*time.Minute {
		t.Fatalf("Now moved by %v, want 20m", got)
	}
}

func newBeatsFor(t *testing.T) *Beats {
	t.Helper()
	return NewBeats()
}

func TestBeats_TickerHandsOutTicksAndReportsItsStopAndPeriod(t *testing.T) {
	b := newBeatsFor(t)
	if b.Period() != 0 {
		t.Fatalf("Period = %v before any ticker, want 0", b.Period())
	}
	ch, stop := b.Ticker(5 * time.Minute)
	if b.Period() != 5*time.Minute {
		t.Fatalf("Period = %v, want 5m", b.Period())
	}
	got := make(chan time.Time, 1)
	go func() { got <- <-ch }()
	at := time.Date(2026, 11, 21, 1, 0, 0, 0, time.UTC)
	b.Tick(t, at)
	if v := <-got; !v.Equal(at) {
		t.Fatalf("tick = %v, want %v", v, at)
	}
	stop()
	b.WaitStopped(t)
}

func pollTwice(p *Probe) {
	for i := 0; i < 2; i++ {
		p.Alive(1)
	}
}

func TestProbe_CountsPollsThatAWaiterMakes(t *testing.T) {
	p := NewProbe()
	for i := 0; i < 3; i++ {
		if !p.Alive(1) {
			t.Fatal("Alive = false, want true")
		}
	}
	p.Drain()
	go pollTwice(p)
	p.WaitPolls(t, 2)
}

func TestProbe_KeepsPollsMadeBeforeWaitPollsIsCalled(t *testing.T) {
	p := NewProbe()
	p.Drain()
	done := make(chan struct{})
	go func() {
		pollTwice(p)
		close(done)
	}()
	<-done
	p.WaitPolls(t, 2)
}

func TestLanes_StepMovesTheClockTicksTheHolderAndWaitsForTwoFreshPolls(t *testing.T) {
	c, b, p := NewClock(), NewBeats(), NewProbe()
	ch, _ := b.Ticker(time.Minute)
	start := c.Now()
	ticked := make(chan time.Time, 1)
	early := make(chan bool, 1)
	stepped := make(chan struct{})
	go func() {
		for c.Now().Equal(start) {
			runtime.Gosched()
		}
		p.Alive(1)
		p.Alive(1)
		ticked <- <-ch
		for i := 0; i < 2; i++ {
			select {
			case <-stepped:
				early <- true
				return
			case <-time.After(100 * time.Millisecond):
			}
			p.Alive(1)
		}
		early <- false
	}()
	Lanes{Clock: c, Holder: b, Waiter: p}.Step(t, 5*time.Minute)
	close(stepped)
	if <-early {
		t.Fatal("Step returned before two fresh polls")
	}
	if got := c.Now().Sub(start); got != 5*time.Minute {
		t.Fatalf("clock moved %v, want 5m", got)
	}
	if at := <-ticked; !at.Equal(c.Now()) {
		t.Fatalf("tick = %v, want %v", at, c.Now())
	}
}

func TestProbe_DrainDiscardsEarlierPolls(t *testing.T) {
	p := NewProbe()
	p.Alive(1)
	p.Drain()
	if got := failure(t, func(tb testing.TB) {
		prev := waitLimit
		waitLimit = 10 * time.Millisecond
		defer func() { waitLimit = prev }()
		p.WaitPolls(tb, 1)
	}); !strings.Contains(got, "polled 0 time(s)") {
		t.Fatalf("message = %q, want a drained probe to see no poll", got)
	}
}

type recordingTB struct {
	testing.TB
	message string
}

type stopped struct{}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Fatal(args ...any) {
	r.message = fmt.Sprint(args...)
	panic(stopped{})
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.message = fmt.Sprintf(format, args...)
	panic(stopped{})
}

func failure(t *testing.T, run func(tb testing.TB)) string {
	t.Helper()
	r := &recordingTB{TB: t}
	func() {
		defer func() {
			if v := recover(); v != nil && v != (stopped{}) {
				panic(v)
			}
		}()
		run(r)
	}()
	return r.message
}

func TestBeats_TickWithoutATickerFailsLoudly(t *testing.T) {
	if got := failure(t, func(tb testing.TB) { NewBeats().Tick(tb, time.Time{}) }); !strings.Contains(got, "no ticker") {
		t.Fatalf("message = %q", got)
	}
}

func TestWaits_GiveUpAfterTheLimitWithAMessage(t *testing.T) {
	prev := waitLimit
	waitLimit = 10 * time.Millisecond
	t.Cleanup(func() { waitLimit = prev })
	b := NewBeats()
	b.Ticker(time.Minute)
	for name, tc := range map[string]struct {
		run  func(tb testing.TB)
		want string
	}{
		"tick":    {func(tb testing.TB) { b.Tick(tb, time.Time{}) }, "did not take the tick"},
		"stopped": {func(tb testing.TB) { b.WaitStopped(tb) }, "were not stopped"},
		"polls":   {func(tb testing.TB) { NewProbe().WaitPolls(tb, 1) }, "polled 0 time(s)"},
	} {
		if got := failure(t, tc.run); !strings.Contains(got, tc.want) {
			t.Errorf("%s: message = %q, want %q", name, got, tc.want)
		}
	}
}
