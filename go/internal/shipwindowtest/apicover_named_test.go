package shipwindowtest

import (
	"fmt"
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
	go pollTwice(p)
	p.WaitPolls(t, 2)
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
