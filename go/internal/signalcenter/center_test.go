package signalcenter

// center_test.go — the Center's delivery contract (ADR-0101 decision 3; design §6).
//
// Delivery model: Emit assigns the sequence number and enqueues under the emit
// lock; the first emitter that finds no drain in progress drains the queue in
// seq order, delivering each event to a snapshot of the listeners, outside the
// lock. A listener that emits (re-entrancy) enqueues and returns; its event is
// delivered by the active drainer right after the current fan-out, before the
// outer Emit returns. A concurrent emitter that finds a drain in progress
// enqueues and returns; the active drainer delivers its event in seq order
// before the drainer's own Emit returns. Every listener therefore observes the
// same total order, and `seq` proves it in the durable record.

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 9, 13, 17, 46, 2, 114000000, time.UTC) }
}

func infoEvent(reason string) Event {
	return Event{Module: ModuleLoop, Origin: "Batch.run", Kind: KindLoopWave, Severity: SeverityInfo, Reason: reason}
}

func TestNew_DefaultsAndOptions(t *testing.T) {
	t.Parallel()
	c := New(WithClock(fixedClock()), WithPID(4242), WithRecentLimit(3))
	var got []Event
	c.Subscribe(func(e Event) { got = append(got, e) })
	c.Emit(infoEvent("one"))
	if len(got) != 1 {
		t.Fatalf("delivered %d events, want 1", len(got))
	}
	e := got[0]
	if e.SchemaVersion != SchemaVersion || e.Seq != 1 || e.PID != 4242 || e.TS != "2026-09-13T17:46:02.114Z" {
		t.Errorf("Emit must stamp schema_version, seq (from 1), pid and the clock's RFC3339Nano UTC: %+v", e)
	}
	for _, r := range []string{"two", "three", "four"} {
		c.Emit(infoEvent(r))
	}
	recent := c.Recent()
	if len(recent) != 3 || recent[0].Reason != "two" || recent[2].Reason != "four" {
		t.Errorf("Recent keeps the last N (3), newest last: %+v", recent)
	}
	d := New()
	d.Emit(infoEvent("x"))
	if e := d.Recent()[0]; e.PID != 0 && e.TS == "" {
		t.Errorf("defaults: pid from the option only, TS from the wall clock: %+v", e)
	}
}

func TestEmit_AllListenersObserveTheSameOrderUnderConcurrency(t *testing.T) {
	t.Parallel()
	c := New()
	const listeners, emitters, perEmitter = 3, 8, 50
	seen := make([][]uint64, listeners)
	var mu sync.Mutex
	for i := 0; i < listeners; i++ {
		i := i
		c.Subscribe(func(e Event) { mu.Lock(); seen[i] = append(seen[i], e.Seq); mu.Unlock() })
	}
	var wg sync.WaitGroup
	for g := 0; g < emitters; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perEmitter; j++ {
				c.Emit(infoEvent("concurrent"))
			}
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	want := emitters * perEmitter
	for i := range seen {
		if len(seen[i]) != want {
			t.Fatalf("listener %d saw %d events, want %d (nothing dropped, every emit delivered before the last Emit returned)", i, len(seen[i]), want)
		}
		for j := 1; j < len(seen[i]); j++ {
			if seen[i][j] <= seen[i][j-1] {
				t.Fatalf("listener %d saw seq %d after %d — delivery must be in seq order", i, seen[i][j], seen[i][j-1])
			}
		}
		if i > 0 && !equalSeqs(seen[0], seen[i]) {
			t.Fatalf("listeners 0 and %d observed different orders", i)
		}
	}
}

func equalSeqs(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEmit_ReentrantListenerIsQueuedNotDeadlocked(t *testing.T) {
	t.Parallel()
	c := New()
	var order []string
	c.Subscribe(func(e Event) {
		order = append(order, e.Reason)
		if e.Reason == "outer" {
			c.Emit(infoEvent("nested")) // re-entrant: must not deadlock, must be delivered after the outer
		}
	})
	done := make(chan struct{})
	go func() { c.Emit(infoEvent("outer")); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("re-entrant Emit deadlocked")
	}
	if strings.Join(order, ",") != "outer,nested" {
		t.Errorf("nested event must follow the outer one, got %v", order)
	}
	recent := c.Recent()
	if len(recent) != 2 || recent[1].Seq != recent[0].Seq+1 {
		t.Errorf("seq proves the order: %+v", recent)
	}
}

func TestEmit_ListenerPanicIsIsolatedReportedAndTheListenerDropped(t *testing.T) {
	t.Parallel()
	c := New()
	calls := 0
	c.Subscribe(func(Event) { calls++; panic("boom") })
	var survivor []Event
	c.Subscribe(func(e Event) { survivor = append(survivor, e) })
	c.Emit(infoEvent("first"))
	c.Emit(infoEvent("second"))
	if calls != 1 {
		t.Errorf("a panicking listener is dropped after its first panic, got %d calls", calls)
	}
	if len(survivor) != 3 {
		t.Fatalf("the surviving listener must see first, the INCIDENT report, second — got %d: %+v", len(survivor), survivor)
	}
	rep := survivor[1]
	if rep.Kind != KindListenerPanicked || rep.Severity != SeverityIncident || rep.Code != CodeListenerPanicked || rep.Module != ModuleSignalCenter {
		t.Errorf("the report is a signalcenter.listener_panicked INCIDENT with its code: %+v", rep)
	}
	if !strings.Contains(rep.Reason, "boom") || !strings.Contains(rep.Fields["listener"], "TestEmit_ListenerPanicIsIsolatedReportedAndTheListenerDropped") || rep.Fields["listener_id"] != "1" {
		t.Errorf("the report names the panic, the listener FUNCTION (not a counter) and its id: %+v", rep)
	}
	if survivor[2].Reason != "second" || survivor[2].Seq != rep.Seq+1 {
		t.Errorf("delivery continues in order after the report: %+v", survivor[2])
	}
}

func TestSubscribe_UnsubscribeIsIdempotentAndSafeDuringEmit(t *testing.T) {
	t.Parallel()
	c := New()
	var a, b int
	unsubA := c.Subscribe(func(Event) { a++ })
	var unsubB func()
	unsubB = c.Subscribe(func(Event) { b++; unsubB() }) // unsubscribes itself mid-fan-out
	c.Emit(infoEvent("one"))
	c.Emit(infoEvent("two"))
	if a != 2 || b != 1 {
		t.Errorf("a=%d b=%d: b must stop after unsubscribing itself during delivery", a, b)
	}
	unsubA()
	unsubA()
	c.Emit(infoEvent("three"))
	if a != 2 {
		t.Errorf("unsubscribe is idempotent and effective, a=%d", a)
	}
}

func TestNilCenter_IsANullObject(t *testing.T) {
	t.Parallel()
	var c *Center
	unsub := c.Subscribe(func(Event) { t.Fatal("a nil center never delivers") })
	unsub()
	c.Emit(infoEvent("ignored"))
	if got := c.Recent(); got != nil {
		t.Errorf("nil center Recent() = %v, want nil", got)
	}
	if c.NDJSONSink(func(int) string { return "" }) == nil {
		t.Error("a nil center still returns a usable no-op sink so producers and roots never branch on nil")
	}
}

func TestEmit_NeverDrops_ValidationRewritesAndRaises(t *testing.T) {
	t.Parallel()
	c := New()
	var got []Event
	c.Subscribe(func(e Event) { got = append(got, e) })
	bad := Event{Module: "core", Origin: "x", Kind: KindPhaseOutcome, Severity: SeverityInfo, Reason: "phase done"}
	c.Emit(bad)
	if len(got) != 1 {
		t.Fatalf("a malformed event is delivered, not dropped: %d", len(got))
	}
	e := got[0]
	if e.Code != CodeUnknownModule || e.Module != ModuleSignalCenter || e.Severity != SeverityWarn || e.Fields["raw_module"] != "core" || e.Seq != 1 {
		t.Errorf("stamped, raised, raw kept, seq assigned: %+v", e)
	}
}

func TestRecent_IsASnapshot(t *testing.T) {
	t.Parallel()
	c := New(WithRecentLimit(2))
	c.Emit(infoEvent("a"))
	snap := c.Recent()
	c.Emit(infoEvent("b"))
	c.Emit(infoEvent("c"))
	if len(snap) != 1 || snap[0].Reason != "a" {
		t.Errorf("Recent returns a copy, not a live window: %+v", snap)
	}
	if r := c.Recent(); len(r) != 2 || r[0].Reason != "b" || r[1].Reason != "c" {
		t.Errorf("bounded window newest last: %+v", r)
	}
}
