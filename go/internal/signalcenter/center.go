package signalcenter

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Listener receives every event the Center delivers. Listeners observe; they
// never decide (ADR-0101 decision 5). A listener may call Emit (re-entrancy is
// queued, never a deadlock). A listener that panics is unsubscribed and the
// panic is reported to the others as a signalcenter.listener_panicked INCIDENT.
type Listener func(Event)

// Option configures a Center at construction.
type Option func(*Center)

// WithClock injects the clock that stamps TS (tests pin it).
func WithClock(now func() time.Time) Option {
	return func(c *Center) {
		if now != nil {
			c.now = now
		}
	}
}

// WithRecentLimit bounds the in-memory window Recent returns (minimum 1).
func WithRecentLimit(n int) Option {
	return func(c *Center) {
		if n < 1 {
			n = 1
		}
		c.recentLimit = n
	}
}

// WithPID stamps every event with the emitting process id, so a resumed
// cycle's second process stays distinguishable in the durable record.
func WithPID(pid int) Option { return func(c *Center) { c.pid = pid } }

type listenerEntry struct {
	id   uint64
	name string
	fn   Listener
}

// listenerName is the function the runtime spells for l — e.g.
// "core.(*Orchestrator).observeSignal-fm" or "signalcenter.(*Center).NDJSONSink.func1"
// — so a listener_panicked INCIDENT names what was dropped, not a counter.
func listenerName(l Listener) string {
	name := runtime.FuncForPC(reflect.ValueOf(l).Pointer()).Name()
	return name[strings.LastIndex(name, "/")+1:]
}

// Center is the process-scoped fan-out (design §6). Delivery model: Emit
// normalizes, stamps the sequence number and enqueues under c.mu; the first
// emitter that finds no drain in progress drains the queue in seq order,
// delivering each event to a snapshot of the listeners outside the lock. An
// emitter that finds a drain in progress — a re-entrant listener, or a
// concurrent goroutine — enqueues and returns; the active drainer delivers its
// event in order before the drainer's own Emit returns. Every listener thus
// observes the same total order, and Seq proves it in the durable record.
// A nil *Center is a Null Object: every method is safe and does nothing.
type Center struct {
	mu          sync.Mutex
	listeners   []listenerEntry
	nextID      uint64
	seq         uint64
	queue       []Event
	draining    bool
	recent      []Event
	recentLimit int
	now         func() time.Time
	pid         int
}

// New constructs a Center with the wall clock, no pid and a 256-event window.
func New(opts ...Option) *Center {
	c := &Center{now: time.Now, recentLimit: 256}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Subscribe registers l and returns its idempotent unsubscribe.
func (c *Center) Subscribe(l Listener) func() {
	if c == nil || l == nil {
		return func() {}
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.listeners = append(c.listeners, listenerEntry{id: id, name: listenerName(l), fn: l})
	c.mu.Unlock()
	return func() { c.unsubscribe(id) }
}

func (c *Center) unsubscribe(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.listeners[:0]
	for _, l := range c.listeners {
		if l.id != id {
			kept = append(kept, l)
		}
	}
	c.listeners = kept
}

func (c *Center) subscribed(id uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.listeners {
		if l.id == id {
			return true
		}
	}
	return false
}

// Emit normalizes e, stamps it and delivers it to every listener in seq order.
// It never drops (see Normalize) and never blocks a listener on itself.
func (c *Center) Emit(e Event) {
	if c == nil {
		return
	}
	e, _ = Normalize(e)
	c.mu.Lock()
	c.seq++
	e.Seq, e.PID, e.SchemaVersion = c.seq, c.pid, SchemaVersion
	e.TS = c.now().UTC().Format(time.RFC3339Nano)
	c.pushRecent(e)
	c.queue = append(c.queue, e)
	if c.draining {
		c.mu.Unlock()
		return
	}
	c.draining = true
	c.drain() // releases c.mu
}

// drain delivers queued events in order; called with c.mu held, returns with
// it released. Listeners run outside the lock so they may subscribe,
// unsubscribe or emit.
func (c *Center) drain() {
	for len(c.queue) > 0 {
		next := c.queue[0]
		c.queue = c.queue[1:]
		snapshot := append([]listenerEntry(nil), c.listeners...)
		c.mu.Unlock()
		c.deliver(next, snapshot)
		c.mu.Lock()
	}
	c.draining = false
	c.mu.Unlock()
}

func (c *Center) deliver(e Event, listeners []listenerEntry) {
	for _, l := range listeners {
		if c.subscribed(l.id) {
			c.call(e, l)
		}
	}
}

// call invokes one listener with panic isolation: a panicking listener is
// unsubscribed and the panic is reported to the others.
func (c *Center) call(e Event, l listenerEntry) {
	defer func() {
		if r := recover(); r != nil {
			c.unsubscribe(l.id)
			c.Emit(Event{
				Module: ModuleSignalCenter, Origin: "Center.deliver", Kind: KindListenerPanicked,
				Code: CodeListenerPanicked, Severity: SeverityIncident,
				Reason: fmt.Sprintf("listener panicked and was unsubscribed: %v", r),
				Fields: map[string]string{"listener": l.name, "listener_id": strconv.FormatUint(l.id, 10)},
				Cycle:  e.Cycle, RunID: e.RunID, Phase: e.Phase,
			})
		}
	}()
	l.fn(e)
}

func (c *Center) pushRecent(e Event) {
	c.recent = append(c.recent, e)
	if len(c.recent) > c.recentLimit {
		c.recent = c.recent[len(c.recent)-c.recentLimit:]
	}
}

// Recent returns a copy of the bounded window, newest last (nil for a nil
// Center) — the cross-cycle view; the durable record is the NDJSON sink.
func (c *Center) Recent() []Event {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.recent...)
}
