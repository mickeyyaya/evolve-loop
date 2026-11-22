// Package shipwindowtest is the fake clock, the manual heartbeat ticker and the poll probe that the tests of the ship-window lease share.
// See docs/architecture/packages/internal-shipwindowtest.md.
package shipwindowtest

import (
	"sync"
	"testing"
	"time"
)

var waitLimit = 30 * time.Second

type Clock struct {
	mu  sync.Mutex
	now time.Time
}

func NewClock() *Clock {
	return &Clock{now: time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)}
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type Beats struct {
	mu      sync.Mutex
	chans   []chan time.Time
	periods []time.Duration
	stopped chan struct{}
}

func NewBeats() *Beats {
	return &Beats{stopped: make(chan struct{}, 64)}
}

func (b *Beats) Ticker(d time.Duration) (<-chan time.Time, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan time.Time)
	b.chans = append(b.chans, ch)
	b.periods = append(b.periods, d)
	return ch, func() { b.stopped <- struct{}{} }
}

func (b *Beats) Period() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.periods) == 0 {
		return 0
	}
	return b.periods[len(b.periods)-1]
}

func (b *Beats) Tick(tb testing.TB, at time.Time) {
	tb.Helper()
	b.mu.Lock()
	if len(b.chans) == 0 {
		b.mu.Unlock()
		tb.Fatal("no ticker was created: the lease starts no heartbeat")
	}
	ch := b.chans[len(b.chans)-1]
	b.mu.Unlock()
	select {
	case ch <- at:
	case <-time.After(waitLimit):
		tb.Fatal("the heartbeat goroutine did not take the tick")
	}
}

func (b *Beats) WaitStopped(tb testing.TB) {
	tb.Helper()
	select {
	case <-b.stopped:
	case <-time.After(waitLimit):
		tb.Fatal("the heartbeat goroutine and its ticker were not stopped")
	}
}

type Probe struct {
	polls chan struct{}
}

func NewProbe() *Probe {
	return &Probe{polls: make(chan struct{}, 4096)}
}

func (p *Probe) Alive(int) bool {
	select {
	case p.polls <- struct{}{}:
	default:
	}
	return true
}

func (p *Probe) Drain() {
	for len(p.polls) > 0 {
		<-p.polls
	}
}

func (p *Probe) WaitPolls(tb testing.TB, n int) {
	tb.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-p.polls:
		case <-time.After(waitLimit):
			tb.Fatalf("the waiter polled %d time(s), want %d", i, n)
		}
	}
}

type Lanes struct {
	Clock  *Clock
	Holder *Beats
	Waiter *Probe
}

func (l Lanes) Step(tb testing.TB, d time.Duration) {
	tb.Helper()
	l.Clock.Advance(d)
	l.Holder.Tick(tb, l.Clock.Now())
	l.Waiter.Drain()
	l.Waiter.WaitPolls(tb, 2)
}
