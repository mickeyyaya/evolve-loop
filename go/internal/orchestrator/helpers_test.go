package orchestrator

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var epoch = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

type fakeTimer struct {
	clock   *fakeClock
	at      time.Time
	f       func()
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	was := !t.stopped
	t.stopped = true
	return was
}

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

func newFakeClock() *fakeClock { return &fakeClock{now: epoch} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clock: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

func (c *fakeClock) armed() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.stopped {
			n++
		}
	}
	return n
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
	for {
		t := c.nextDue()
		if t == nil {
			return
		}
		t.f()
	}
}

func (c *fakeClock) nextDue() *fakeTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	sort.SliceStable(c.timers, func(i, j int) bool { return c.timers[i].at.Before(c.timers[j].at) })
	for _, t := range c.timers {
		if !t.stopped && !t.at.After(c.now) {
			t.stopped = true
			return t
		}
	}
	return nil
}

type eventLog struct {
	mu     sync.Mutex
	events []signalcenter.Event
}

func (l *eventLog) publish(e signalcenter.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *eventLog) withCode(code signalcenter.Code) []signalcenter.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []signalcenter.Event
	for _, e := range l.events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

type claimLog struct {
	mu       sync.Mutex
	released []string
	parked   []string
	reasons  []string
}

func (c *claimLog) Release(todo, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.released = append(c.released, todo)
	c.reasons = append(c.reasons, reason)
	return nil
}

func (c *claimLog) Park(todo, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parked = append(c.parked, todo)
	c.reasons = append(c.reasons, reason)
	return nil
}

func (c *claimLog) snapshot() (released, parked, reasons []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.released...), append([]string(nil), c.parked...), append([]string(nil), c.reasons...)
}

type gatedLaunch struct {
	started chan string
	mu      sync.Mutex
	release map[string]chan int
}

func newGatedLaunch() *gatedLaunch {
	return &gatedLaunch{started: make(chan string, 64), release: map[string]chan int{}}
}

func (g *gatedLaunch) gate(id string) chan int {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch, ok := g.release[id]
	if !ok {
		ch = make(chan int, 4)
		g.release[id] = ch
	}
	return ch
}

func (g *gatedLaunch) launch(ctx context.Context, spec fleet.CycleSpec) (int, error) {
	id := spec.Scope[0]
	gate := g.gate(id)
	g.started <- id
	select {
	case rc := <-gate:
		return rc, nil
	case <-ctx.Done():
		return 143, nil
	}
}

func (g *gatedLaunch) finish(id string, rc int) { g.gate(id) <- rc }

func waitStart(t *testing.T, g *gatedLaunch) string {
	t.Helper()
	select {
	case id := <-g.started:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a lane launch")
		return ""
	}
}

func noHeartbeat(Lane) (time.Time, bool) { return time.Time{}, false }

func silentHeartbeat(Lane) (time.Time, bool) { return time.Time{}, true }
