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

type Listener func(Event)

type Option func(*Center)

func WithClock(now func() time.Time) Option {
	return func(c *Center) {
		if now != nil {
			c.now = now
		}
	}
}

func WithRecentLimit(n int) Option {
	return func(c *Center) {
		if n < 1 {
			n = 1
		}
		c.recentLimit = n
	}
}

func WithPID(pid int) Option { return func(c *Center) { c.pid = pid } }

type listenerEntry struct {
	id   uint64
	name string
	fn   Listener
}

func listenerName(l Listener) string {
	name := runtime.FuncForPC(reflect.ValueOf(l).Pointer()).Name()
	return name[strings.LastIndex(name, "/")+1:]
}

type Center struct {
	mu          sync.Mutex
	listeners   []listenerEntry
	nextID      uint64
	seq         uint64
	queue       []Event
	draining    bool
	idle        *sync.Cond
	recent      []Event
	recentLimit int
	now         func() time.Time
	pid         int
}

const defaultRecentLimit = 256

func New(opts ...Option) *Center {
	c := &Center{now: time.Now, recentLimit: defaultRecentLimit}
	c.idle = sync.NewCond(&c.mu)
	for _, opt := range opts {
		opt(c)
	}
	return c
}

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
	c.drain()
}

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
	c.idle.Broadcast()
	c.mu.Unlock()
}

func (c *Center) Flush() {
	if c == nil {
		return
	}
	c.mu.Lock()
	for c.draining || len(c.queue) > 0 {
		c.idle.Wait()
	}
	c.mu.Unlock()
}

func (c *Center) deliver(e Event, listeners []listenerEntry) {
	for _, l := range listeners {
		if c.subscribed(l.id) {
			c.call(e, l)
		}
	}
}

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

func (c *Center) Recent() []Event {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.recent...)
}
